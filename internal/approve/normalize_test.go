package approve

import (
	"errors"
	"testing"
)

// TestNormalize_QuoteAndWhitespaceInvariance verifies that quote/whitespace
// variations of the same command yield the same hash. This is the invariant
// operators rely on: `git push origin main` normalizes identically whether
// typed with single/double quotes or extra spaces.
func TestNormalize_QuoteAndWhitespaceInvariance(t *testing.T) {
	forms := []string{
		`git push origin main`,
		`git  push  origin  main`,
		`git push "origin" main`,
		`git push 'origin' main`,
		`git push "origin" "main"`,
		"git push origin main\n",
	}
	first, _, err := Normalize(forms[0])
	if err != nil {
		t.Fatalf("baseline normalize failed: %v", err)
	}
	for _, form := range forms[1:] {
		got, _, err := Normalize(form)
		if err != nil {
			t.Fatalf("normalize(%q) failed: %v", form, err)
		}
		if got != first {
			t.Errorf("hash mismatch for %q: got %s, want %s", form, got, first)
		}
	}
}

// TestNormalize_SemanticDistinctness verifies that semantically distinct
// commands never collide on the normalized hash.
func TestNormalize_SemanticDistinctness(t *testing.T) {
	cases := []string{
		`rm -rf /`,
		`rm -rf /tmp`,
		`rm -f /`,
		`ls -rf /`,
		`echo hello`,
		`echo hello world`,
	}
	seen := map[string]string{}
	for _, cmd := range cases {
		h, _, err := Normalize(cmd)
		if err != nil {
			t.Fatalf("normalize(%q): %v", cmd, err)
		}
		if prev, ok := seen[h]; ok {
			t.Errorf("hash collision: %q and %q both hash to %s", prev, cmd, h)
		}
		seen[h] = cmd
	}
}

// TestNormalize_DynamicRejected verifies that any command containing a
// runtime expansion is rejected — approving such a command would tie the
// approval to a representation that no longer matches when the expansion
// changes.
func TestNormalize_DynamicRejected(t *testing.T) {
	cases := []string{
		`echo $HOME`,
		`echo ${USER}`,
		`echo "$USER"`,
		`echo $(whoami)`,
		"echo `whoami`",
		`echo "$(rm -rf /)"`,
		`cat <(ls)`,
		`echo $((1 + 2))`,
	}
	for _, cmd := range cases {
		_, _, err := Normalize(cmd)
		if !errors.Is(err, ErrDynamicCommand) {
			t.Errorf("Normalize(%q): want ErrDynamicCommand, got %v", cmd, err)
		}
	}
}

// TestNormalize_Empty rejects empty and whitespace-only input.
func TestNormalize_Empty(t *testing.T) {
	for _, cmd := range []string{"", "   ", "\t\n"} {
		_, _, err := Normalize(cmd)
		if !errors.Is(err, ErrEmptyCommand) {
			t.Errorf("Normalize(%q): want ErrEmptyCommand, got %v", cmd, err)
		}
	}
}

// TestNormalize_ArgvBoundaryPreserved verifies that commands whose argv
// shape differs — e.g. `echo "a b"` (1 arg) vs `echo a b` (2 args) — never
// collide on the normalized hash. This is skeptic C2: the space-joined
// canonical form let an approval of `echo a b` be redeemed against
// `echo "a b"` (a semantically different command).
func TestNormalize_ArgvBoundaryPreserved(t *testing.T) {
	cases := [][2]string{
		{`echo "a b"`, `echo a b`},
		{`find "-name x"`, `find -name x`},
		{`printf "%s %s" a b`, `printf %s %s a b`},
		{`git commit -m "one two"`, `git commit -m one two`},
	}
	for _, pair := range cases {
		h1, _, err := Normalize(pair[0])
		if err != nil {
			t.Fatalf("Normalize(%q): %v", pair[0], err)
		}
		h2, _, err := Normalize(pair[1])
		if err != nil {
			t.Fatalf("Normalize(%q): %v", pair[1], err)
		}
		if h1 == h2 {
			t.Errorf("argv boundary collision: %q and %q both hash to %s", pair[0], pair[1], h1)
		}
	}
}

// TestNormalize_ForClauseUnsupported verifies that ForClause / WhileClause /
// IfClause / CaseClause / FuncDecl return ErrUnsupported, giving the hook an
// explicit signal to attach a human-readable reason to the deny. Regression
// for skeptic C1 (silent Normalize failures dropped the deny promise).
func TestNormalize_ForClauseUnsupported(t *testing.T) {
	cases := []string{
		`for f in a b; do rm $f; done`,
		`while true; do echo hi; done`,
		`if true; then echo hi; fi`,
		`case $x in y) echo hi;; esac`,
		`myfn() { echo hi; }`,
	}
	for _, cmd := range cases {
		_, _, err := Normalize(cmd)
		if !errors.Is(err, ErrUnsupported) && !errors.Is(err, ErrDynamicCommand) {
			// Some cases parse with a $ inside (case $x, while ...) and will
			// hit ErrDynamicCommand first; that is also an acceptable
			// non-silent error.
			t.Errorf("Normalize(%q): want ErrUnsupported or ErrDynamicCommand, got %v", cmd, err)
		}
	}
}

// TestNormalize_ParseError surfaces mvdan.cc/sh parse errors verbatim.
func TestNormalize_ParseError(t *testing.T) {
	_, _, err := Normalize(`echo "unterminated`)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if errors.Is(err, ErrDynamicCommand) || errors.Is(err, ErrEmptyCommand) {
		t.Errorf("expected raw parse error, got taxonomy error: %v", err)
	}
}
