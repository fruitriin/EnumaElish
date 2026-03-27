// Package audit implements the ccchain rule audit (flat expansion) engine.
package audit

import (
	"fmt"
	"strings"

	"github.com/fruitriin/ccchain/internal/dsl"
)

// AuditOutput represents the full audit result.
type AuditOutput struct {
	Lines    []AuditLine
	Settings *dsl.Settings
	Stats    Stats
}

// AuditLine represents a single line of audit output.
type AuditLine struct {
	Action   dsl.Action
	Command  string // e.g., "find | rm"
	Template string // e.g., "bulkExec"
	Message  string
	Note     string // e.g., "(&&: reset → top-level rm rule)"
}

// Stats holds audit statistics.
type Stats struct {
	RuleCount     int
	TemplateCount int
	Truncated     bool
}

// Audit generates a flat expansion of all rules in the config.
func Audit(config *dsl.Config) *AuditOutput {
	out := &AuditOutput{
		Settings: config.Settings,
		Stats: Stats{
			RuleCount:     len(config.Rules) + len(config.PreRules) + len(config.PostRules),
			TemplateCount: len(config.Templates),
		},
	}

	maxDepth := config.Settings.MaxContextDepth
	maxRules := config.Settings.MaxRulesPerCmd

	rules := append(config.PreRules, config.Rules...)

	for _, rule := range rules {
		// Top-level rule
		for _, cmd := range rule.Commands {
			out.Lines = append(out.Lines, AuditLine{
				Action:  rule.Action,
				Command: cmd,
				Message: rule.Message,
			})
		}

		// Pipe context rules
		pipeRules := collectPipeRules(rule, config)
		ruleCount := 0
		for _, pr := range pipeRules {
			if ruleCount >= maxRules {
				out.Stats.Truncated = true
				break
			}
			for _, cmd := range rule.Commands {
				for _, pipeCmd := range pr.Commands {
					tmpl := templateName(rule, config)
					out.Lines = append(out.Lines, AuditLine{
						Action:   pr.Action,
						Command:  cmd + " | " + pipeCmd,
						Template: tmpl,
						Message:  pr.Message,
					})
				}
			}
			ruleCount++
		}

		// Exec context rules
		execRules := collectExecRules(rule, config)
		for _, er := range execRules {
			for _, cmd := range rule.Commands {
				for _, execCmd := range er.Commands {
					tmpl := templateName(rule, config)
					out.Lines = append(out.Lines, AuditLine{
						Action:   er.Action,
						Command:  cmd + " -exec " + execCmd,
						Template: tmplExecName(tmpl),
						Message:  er.Message,
					})
				}
			}
		}

		// Show reset behavior if depth allows
		if maxDepth >= 2 {
			for _, cmd := range rule.Commands {
				out.Lines = append(out.Lines, AuditLine{
					Action:  "",
					Command: cmd + " && ...",
					Note:    "(&&: reset → top-level rules)",
				})
			}
		}
	}

	return out
}

// Format formats the audit output as a human-readable string.
func Format(out *AuditOutput) string {
	var sb strings.Builder

	for _, line := range out.Lines {
		if line.Action == "" {
			fmt.Fprintf(&sb, "[---]    %-30s %s\n", line.Command, line.Note)
			continue
		}

		actionStr := fmt.Sprintf("[%s]", line.Action)
		tmplStr := ""
		if line.Template != "" {
			tmplStr = fmt.Sprintf("(template: %s)", line.Template)
		}
		msgStr := ""
		if line.Message != "" {
			msgStr = fmt.Sprintf("%q", line.Message)
		}

		parts := []string{fmt.Sprintf("%-8s %-30s", actionStr, line.Command)}
		if tmplStr != "" {
			parts = append(parts, tmplStr)
		}
		if msgStr != "" {
			parts = append(parts, msgStr)
		}

		fmt.Fprintln(&sb, strings.Join(parts, "  "))
	}

	if out.Stats.Truncated {
		fmt.Fprintln(&sb, "--- truncated (max_rules_per_cmd reached) ---")
	}

	fmt.Fprintln(&sb)
	fmt.Fprintln(&sb, "Settings:")
	fmt.Fprintf(&sb, "  max_context_depth: %d\n", out.Settings.MaxContextDepth)
	fmt.Fprintf(&sb, "  max_rules_per_cmd: %d\n", out.Settings.MaxRulesPerCmd)
	fmt.Fprintf(&sb, "  fallback: %s\n", out.Settings.Fallback)

	fmt.Fprintln(&sb)
	fmt.Fprintln(&sb, "Stats:")
	fmt.Fprintf(&sb, "  rules: %d\n", out.Stats.RuleCount)
	fmt.Fprintf(&sb, "  templates: %d\n", out.Stats.TemplateCount)

	return sb.String()
}

func collectPipeRules(rule *dsl.Rule, config *dsl.Config) []*dsl.Rule {
	var rules []*dsl.Rule
	rules = append(rules, rule.PipeRules...)

	if rule.Next != "" {
		tmpl := dsl.LookupTemplate(config, rule.Next)
		if tmpl != nil {
			rules = append(rules, collectTemplatePipeRules(tmpl, config, make(map[string]bool))...)
		}
	}

	return rules
}

func collectExecRules(rule *dsl.Rule, config *dsl.Config) []*dsl.Rule {
	var rules []*dsl.Rule
	rules = append(rules, rule.ExecRules...)

	if rule.Next != "" {
		tmpl := dsl.LookupTemplate(config, rule.Next)
		if tmpl != nil {
			rules = append(rules, collectTemplateExecRules(tmpl, config, make(map[string]bool))...)
		}
	}

	return rules
}

func collectTemplatePipeRules(tmpl *dsl.Template, config *dsl.Config, visited map[string]bool) []*dsl.Rule {
	if visited[tmpl.Name] {
		return nil
	}
	visited[tmpl.Name] = true

	var rules []*dsl.Rule
	rules = append(rules, tmpl.PipeRules...)

	if tmpl.Next != "" {
		nextTmpl := dsl.LookupTemplate(config, tmpl.Next)
		if nextTmpl != nil {
			rules = append(rules, collectTemplatePipeRules(nextTmpl, config, visited)...)
		}
	}

	return rules
}

func collectTemplateExecRules(tmpl *dsl.Template, config *dsl.Config, visited map[string]bool) []*dsl.Rule {
	if visited[tmpl.Name] {
		return nil
	}
	visited[tmpl.Name] = true

	var rules []*dsl.Rule
	rules = append(rules, tmpl.ExecRules...)

	if tmpl.Next != "" {
		nextTmpl := dsl.LookupTemplate(config, tmpl.Next)
		if nextTmpl != nil {
			rules = append(rules, collectTemplateExecRules(nextTmpl, config, visited)...)
		}
	}

	return rules
}

func templateName(rule *dsl.Rule, config *dsl.Config) string {
	if rule.Next != "" {
		return rule.Next
	}
	return ""
}

func tmplExecName(tmpl string) string {
	if tmpl != "" {
		return tmpl + ".exec"
	}
	return ""
}
