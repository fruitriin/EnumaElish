package eval

// ModeClass classifies a Claude Code permission_mode by whether an "ask"
// decision reaches a human as an interactive dialog.
type ModeClass int

const (
	// ModeInteractive: ask decisions surface as a dialog the user answers
	// (default, acceptEdits, plan, bypassPermissions).
	ModeInteractive ModeClass = iota
	// ModeNonInteractive: ask decisions never reach a human
	// (auto: classifier decides; dontAsk: auto-denied). Unknown or empty
	// modes fall here so that future modes fail safe.
	ModeNonInteractive
)

// ClassifyMode maps a permission_mode string from the hook input JSON to a
// ModeClass. Unknown and empty values classify as non-interactive: a mode we
// don't recognize cannot be trusted to show a dialog, and degrading ask is
// the safe direction (deny-first).
func ClassifyMode(mode string) ModeClass {
	switch mode {
	case "default", "acceptEdits", "plan", "bypassPermissions":
		return ModeInteractive
	default:
		return ModeNonInteractive
	}
}
