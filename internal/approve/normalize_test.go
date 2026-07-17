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
