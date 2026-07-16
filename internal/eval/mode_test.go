package eval

import "testing"

func TestClassifyMode(t *testing.T) {
	cases := map[string]ModeClass{
		"default":           ModeInteractive,
		"acceptEdits":       ModeInteractive,
		"plan":              ModeInteractive,
		"bypassPermissions": ModeInteractive,
		"auto":              ModeNonInteractive,
		"dontAsk":           ModeNonInteractive,
		// Forward compatibility: unknown modes must fail safe.
		"":            ModeNonInteractive,
		"newMode2027": ModeNonInteractive,
		"DEFAULT":     ModeNonInteractive, // case-sensitive: exact match only
	}
	for mode, want := range cases {
		if got := ClassifyMode(mode); got != want {
			t.Errorf("ClassifyMode(%q) = %v, want %v", mode, got, want)
		}
	}
}
