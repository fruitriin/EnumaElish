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

// Normalize returns the SHA-256 hex hash of the canonically-serialized
// command together with the canonical form itself.
//
// Canonicalization: the command is parsed with mvdan.cc/sh (bash mode) and
// re-serialized with quote-stripping and whitespace collapsing so that
// `echo hi`, `echo 'hi'`, `echo "hi"`, and `echo  hi` all hash identically
// while `rm -rf /` and `rm -rf /tmp` do not.
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
			sb.WriteString(" ; ")
		}
		if err := canonicalStmt(&sb, stmt); err != nil {
			return "", "", err
		}
	}
	canonical = sb.String()
	if canonical == "" {
		return "", "", ErrUnsupported
	}
	sum := sha256.Sum256([]byte(canonical))
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
func canonicalStmt(sb *strings.Builder, s *syntax.Stmt) error {
	if s == nil {
		return ErrUnsupported
	}
	if s.Negated {
		sb.WriteString("! ")
	}
	if err := canonicalCommand(sb, s.Cmd); err != nil {
		return err
	}
	for _, r := range s.Redirs {
		sb.WriteByte(' ')
		sb.WriteString(r.Op.String())
		sb.WriteByte(' ')
		if err := canonicalWord(sb, r.Word); err != nil {
			return err
		}
	}
	if s.Background {
		sb.WriteString(" &")
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
					sb.WriteByte(' ')
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
				sb.WriteByte(' ')
			}
		}
		for i, w := range v.Args {
			if i > 0 {
				sb.WriteByte(' ')
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
		sb.WriteByte(' ')
		sb.WriteString(v.Op.String())
		sb.WriteByte(' ')
		return canonicalStmt(sb, v.Y)
	case *syntax.Subshell:
		sb.WriteByte('(')
		for i, s := range v.Stmts {
			if i > 0 {
				sb.WriteString(" ; ")
			}
			if err := canonicalStmt(sb, s); err != nil {
				return err
			}
		}
		sb.WriteByte(')')
		return nil
	case *syntax.Block:
		sb.WriteByte('{')
		sb.WriteByte(' ')
		for i, s := range v.Stmts {
			if i > 0 {
				sb.WriteString(" ; ")
			}
			if err := canonicalStmt(sb, s); err != nil {
				return err
			}
		}
		sb.WriteString(" ; }")
		return nil
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
