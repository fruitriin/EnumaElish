// Package dsl implements the ccchain DSL parser.
package dsl

// Config represents the top-level parsed DSL configuration.
type Config struct {
	Templates      []*Template
	TemplateIndex  map[string]*Template // populated by ResolveTemplates
	PreRules       []*Rule              // rules under preToolUse section
	PostRules      []*Rule              // rules under postToolUse section
	Rules          []*Rule              // rules outside any section (legacy/default = preToolUse)
	Settings       *Settings
}

// Action represents the action type of a rule.
type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
	ActionWarn  Action = "warn"
	ActionAsk   Action = "ask"
	ActionHint  Action = "hint"
)

// IsValidAction returns true if the string is a valid action.
func IsValidAction(s string) bool {
	switch Action(s) {
	case ActionAllow, ActionDeny, ActionWarn, ActionAsk, ActionHint:
		return true
	}
	return false
}

// Rule represents a single permission rule.
type Rule struct {
	Action   Action
	Commands []string // one or more command names (e.g., "cat, echo, head")
	Message  string   // optional deny/warn message

	// Nested context rules
	PipeRules []*Rule // rules under |,>> context
	ExecRules []*Rule // rules under exec: context
	ArgsRules []*ArgsRule // rules under args: context

	// Properties
	Mode    string // "block", "warn", "hint"
	Next    string // template delegation

	// Source location for error reporting
	Line int
}

// ArgsRule represents a pattern-based argument rule.
type ArgsRule struct {
	Pattern string // regex pattern
	Action  Action
	Message string
	Line    int
}

// Template represents a reusable rule template.
type Template struct {
	Name    string
	Extends string // parent template name

	PipeRules []*Rule
	ExecRules []*Rule
	ArgsRules []*ArgsRule
	Next      string

	Line int
}

// Settings represents the settings block.
type Settings struct {
	MaxContextDepth int
	MaxRulesPerCmd  int
	Fallback        Action
	Line            int
}

// DefaultSettings returns settings with default values.
func DefaultSettings() *Settings {
	return &Settings{
		MaxContextDepth: 2,
		MaxRulesPerCmd:  5,
		Fallback:        ActionAsk,
	}
}
