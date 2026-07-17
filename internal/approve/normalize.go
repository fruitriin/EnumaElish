// Package approve implements the ccchain approval-token flow (Plan 0022 Phase 3).
//
// The flow lets a human asynchronously approve an ask rule that degraded to
// deny under a non-interactive permission mode: the hook records the pending
// request, the human runs `ccchain approve --last` in their own terminal, and
// the next hook call for the same command consumes the approval.
//
// Threat model (must never be violated):
//
//   - The hint returned to the agent MUST NOT carry the token or hash. It
//     lands in the agent's context; a token there lets the agent self-approve.
//   - `ccchain approve` itself must be denied to the agent (sentinel preset,
//     Phase 4). This package cannot enforce that; it only maintains the store.
package approve

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// ErrDynamicCommand is returned by Normalize when the command contains dynamic
// expansions ($VAR, $(...), backticks, arithmetic, process substitution).
// Such commands are not eligible for approval because their meaning at hook
// time differs from their meaning at execution time (the expansion result
// changes with the environment).
var ErrDynamicCommand = errors.New("command contains dynamic expansion (unsupported for approval)")

// ErrEmptyCommand is returned by Normalize when the input is empty or contains
// only whitespace.
var ErrEmptyCommand = errors.New("empty command")

// ErrUnsupported is returned when the AST shape is outside the analyzable
// subset (e.g. function declarations, case blocks). Approval requires an
// unambiguous single-shot command signature.
var ErrUnsupported = errors.New("command shape unsupported for approval")

// argSep separates argv elements in the canonical form. Chosen as ASCII Unit
// Separator (U+001F, control character): argv strings never contain it in
// practice (POSIX permits any non-NUL byte, but no real shell tool uses U+001F
// as data). This preserves argv boundaries under hashing so that quoted vs
// unquoted whitespace-carrying arguments do NOT collide:
//
//	echo "a b"   → hash H1  (1 arg: "a b")
//	echo a b     → hash H2  (2 args: "a", "b")
//
// The previous space-joined form ("echo a b" for both) let a caller approve
// `echo a b` and then successfully redeem the token against `echo "a b"` (an
// argv-shape-different command). See skeptic C2 in Plan 0022/0025 review.
const argSep = "\x1f"
const stmtSep = "\x1e" // ASCII Record Separator — between top-level statements.

// Normalize returns the SHA-256 hex hash of the canonically-serialized
// command together with the canonical form itself.
//
// Canonicalization: the command is parsed with mvdan.cc/sh (bash mode) and
// re-serialized with quote-stripping so that `echo hi`, `echo 'hi'`,
// `echo "hi"`, and `echo  hi` (single argument, varying quotes/whitespace
// around it) all hash identically, while argv-shape-different commands like
// `echo "a b"` (1 arg) vs `echo a b` (2 args) hash distinctly.
//
// The distinctness of argv shape is preserved by joining argv elements with an
// ASCII Unit Separator (U+001F) that argv strings never contain in practice.
// The human-readable canonical form (returned as `canonical`) uses spaces so
// it remains legible in list/audit output; the hash is computed from the
// unit-separated form.
//
// Dynamic content aborts normalization: variable expansion ($VAR), command
// substitution ($(...) or backticks), process substitution (<(...)) and
// arithmetic ($((...))) all evaluate at runtime, so their pre-execution
// representation cannot be safely tied to their execution semantics.
func Normalize(command string) (hash string, canonical string, err error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return "", "", ErrEmptyCommand
	}
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(command), "")
	if err != nil {
		return "", "", fmt.Errorf("parse: %w", err)
	}
	if containsDynamic(file) {
		return "", "", ErrDynamicCommand
	}
	var sb strings.Builder
	for i, stmt := range file.Stmts {
		if i > 0 {
			sb.WriteString(stmtSep)
		}
		if err := canonicalStmt(&sb, stmt); err != nil {
			return "", "", err
		}
	}
	hashSrc := sb.String()
	if hashSrc == "" {
		return "", "", ErrUnsupported
	}
	sum := sha256.Sum256([]byte(hashSrc))
	// Produce a human-readable form for list/audit output. Argv separators
	// become spaces; statement separators become " ; ".
	displayer := strings.NewReplacer(argSep, " ", stmtSep, " ; ")
	canonical = displayer.Replace(hashSrc)
	return hex.EncodeToString(sum[:]), canonical, nil
}

// containsDynamic walks the syntax tree and reports whether any node carries a
// runtime-evaluated expansion. The walk is exhaustive (including inside
// double-quoted strings) — otherwise a payload like `"$(rm)"` would be
// mistaken for a static string.
func containsDynamic(node syntax.Node) bool {
	found := false
	syntax.Walk(node, func(n syntax.Node) bool {
		if n == nil {
			return false
		}
		switch n.(type) {
		case *syntax.ParamExp, // $var, ${var}
			*syntax.CmdSubst,  // $(cmd), `cmd`
			*syntax.ProcSubst, // <(cmd), >(cmd)
			*syntax.ArithmExp, // $((expr))
			*syntax.ArithmCmd: // ((expr))
			found = true
			return false
		}
		return true
	})
	return found
}

// canonicalStmt serializes a single statement, including its background /
// negation flags and any redirects. Only shapes we intend to support for
// approval are handled — otherwise ErrUnsupported.
//
// Redirects and the redir target are also joined via argSep so that
// `cat foo >x` and `cat "foo >x"` cannot be conflated (the latter has argSep
// between "foo" and ">x" as a single Word; the former has argSep-Op-argSep-x
// as separate tokens).
func canonicalStmt(sb *strings.Builder, s *syntax.Stmt) error {
	if s == nil {
		return ErrUnsupported
	}
	if s.Negated {
		sb.WriteString("!")
		sb.WriteString(argSep)
	}
	if err := canonicalCommand(sb, s.Cmd); err != nil {
		return err
	}
	for _, r := range s.Redirs {
		sb.WriteString(argSep)
		sb.WriteString(r.Op.String())
		sb.WriteString(argSep)
		if err := canonicalWord(sb, r.Word); err != nil {
			return err
		}
	}
	if s.Background {
		sb.WriteString(argSep)
		sb.WriteByte('&')
	}
	return nil
}

// canonicalCommand handles the command shapes we allow: CallExpr (a normal
// argv), BinaryCmd (pipes / && / ||), and Subshell / Block (`(...)` `{...}`).
func canonicalCommand(sb *strings.Builder, c syntax.Command) error {
	switch v := c.(type) {
	case *syntax.CallExpr:
		if len(v.Assigns) > 0 {
			// FOO=bar cmd — assigns are treated as literal-only lit=word pairs.
			for i, a := range v.Assigns {
				if i > 0 {
					sb.WriteString(argSep)
				}
				sb.WriteString(a.Name.Value)
				sb.WriteByte('=')
				if a.Value != nil {
					if err := canonicalWord(sb, a.Value); err != nil {
						return err
					}
				}
			}
			if len(v.Args) > 0 {
				sb.WriteString(argSep)
			}
		}
		for i, w := range v.Args {
			if i > 0 {
				sb.WriteString(argSep)
			}
			if err := canonicalWord(sb, w); err != nil {
				return err
			}
		}
		return nil
	case *syntax.BinaryCmd:
		if err := canonicalStmt(sb, v.X); err != nil {
			return err
		}
		sb.WriteString(argSep)
		sb.WriteString(v.Op.String())
		sb.WriteString(argSep)
		return canonicalStmt(sb, v.Y)
	case *syntax.Subshell:
		sb.WriteByte('(')
		for i, s := range v.Stmts {
			if i > 0 {
				sb.WriteString(stmtSep)
			}
			if err := canonicalStmt(sb, s); err != nil {
				return err
			}
		}
		sb.WriteByte(')')
		return nil
	case *syntax.Block:
		sb.WriteByte('{')
		sb.WriteString(argSep)
		for i, s := range v.Stmts {
			if i > 0 {
				sb.WriteString(stmtSep)
			}
			if err := canonicalStmt(sb, s); err != nil {
				return err
			}
		}
		sb.WriteString(stmtSep)
		sb.WriteByte('}')
		return nil
	// ForClause / WhileClause / IfClause / CaseClause / FuncDecl are explicitly
	// listed to make the "unsupported" answer intentional. When Plan 0025
	// expanded for-loops into per-iteration segments at the evaluate layer, the
	// approval-side normalizer was still receiving the original ForClause AST
	// and silently reporting ErrUnsupported from the default: below — the
	// caller (recordPendingApproval) then dropped the deny message on the
	// floor. Listing them here makes future audits notice that any AST
	// coverage change needs a matching normalizer update. See skeptic C1 in
	// Plan 0022/0025 review.
	case *syntax.ForClause,
		*syntax.WhileClause,
		*syntax.IfClause,
		*syntax.CaseClause,
		*syntax.FuncDecl:
		return ErrUnsupported
	default:
		return ErrUnsupported
	}
}

// canonicalWord renders a Word by concatenating its literal payload — the
// exact string the shell would pass to the command. Dynamic parts are
// already rejected upstream, so only Lit / SglQuoted / DblQuoted appear.
func canonicalWord(sb *strings.Builder, w *syntax.Word) error {
	if w == nil {
		return nil
	}
	for _, part := range w.Parts {
		if err := writeCanonicalPart(sb, part); err != nil {
			return err
		}
	}
	return nil
}

func writeCanonicalPart(sb *strings.Builder, part syntax.WordPart) error {
	switch p := part.(type) {
	case *syntax.Lit:
		sb.WriteString(p.Value)
	case *syntax.SglQuoted:
		// $'...' is a dynamic escape form; containsDynamic would flag any
		// enclosed expansion, but the ANSI-C escapes themselves change meaning
		// at runtime — refuse to normalize.
		if p.Dollar {
			return ErrUnsupported
		}
		sb.WriteString(p.Value)
	case *syntax.DblQuoted:
		for _, inner := range p.Parts {
			if err := writeCanonicalPart(sb, inner); err != nil {
				return err
			}
		}
	default:
		return ErrUnsupported
	}
	return nil
}
