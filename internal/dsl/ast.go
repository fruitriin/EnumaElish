// Package dsl implements the ccchain DSL parser.
package dsl

import "regexp"

// Config represents the top-level parsed DSL configuration.
type Config struct {
	Templates     []*Template
	TemplateIndex map[string]*Template // populated by ResolveTemplates
	PreRules      []*Rule              // rules under preToolUse section
	PostRules     []*Rule              // rules under postToolUse section
	Rules         []*Rule              // rules outside any section (legacy/default = preToolUse)
	Settings      *Settings
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
	PipeRules []*Rule     // rules under |,>> context
	ExecRules []*Rule     // rules under exec: context
	ArgsRules []*ArgsRule // rules under args: context
	ScopeRule *ScopeRule  // rule under scope: context (Plan 0011 v2)

	// Properties
	Mode string // "block", "warn", "hint"
	Next string // template delegation

	// Source location for error reporting
	Line int
}

// ScopeRule describes per-scope actions for a rule (Plan 0011 v2).
// Fields are pointers so unset entries can be distinguished from explicit "allow".
//
// Precedence when classifying a path:
//   - inside path → Inside (or the rule's base action if Inside is nil)
//   - outside path used as read arg  → OutsideRead, else Outside
//   - outside path used as write arg → OutsideWrite, else Outside
//
// Backward compatibility: writing only `outside:` applies to both read and write.
type ScopeRule struct {
	Inside       *ScopeAction
	Outside      *ScopeAction // fallback for both read and write when specific ones not set
	OutsideRead  *ScopeAction
	OutsideWrite *ScopeAction
	Line         int
}

// ScopeAction is a scope-clause action + optional message.
type ScopeAction struct {
	Action  Action
	Message string
	Line    int
}

// ArgsRule represents a pattern-based argument rule.
type ArgsRule struct {
	Pattern  string // regex pattern
	Action   Action
	Message  string
	Line     int
	Compiled *regexp.Regexp // pre-compiled regex, set by ValidateArgsRules
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
//
// Explicit tracks which fields the user set in a settings: block (keyed by DSL
// key name — e.g. "fallback", "workspace", "strict_config_error"). Field-wise
// merging in mergeConfigs relies on this to decide whether an overlay's
// Settings field should override the base. Without it, an overlay `settings:`
// block that touches one field silently blanks all the others (Plan 0006 C5).
type Settings struct {
	MaxContextDepth   int
	MaxRulesPerCmd    int
	Fallback          Action
	WorkspacePaths    []string // scope: workspace paths
	StrictConfigError bool     // if true, hook denies when config load fails
	Line              int

	// Explicit fields set by the parser (keyed by DSL key name).
	Explicit map[string]bool
}

// DefaultSettings returns settings with default values.
func DefaultSettings() *Settings {
	return &Settings{
		MaxContextDepth: 2,
		MaxRulesPerCmd:  5,
		Fallback:        ActionAsk,
		Explicit:        map[string]bool{},
	}
}
