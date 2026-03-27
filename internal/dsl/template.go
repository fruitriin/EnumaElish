package dsl

import "fmt"

// ResolveTemplates resolves all template references (extends and next) in the config.
// It returns an error if circular references are detected.
func ResolveTemplates(config *Config) error {
	tmplMap := make(map[string]*Template, len(config.Templates))
	for _, t := range config.Templates {
		if _, exists := tmplMap[t.Name]; exists {
			return &ParseError{Line: t.Line, Message: fmt.Sprintf("duplicate template: %q", t.Name)}
		}
		tmplMap[t.Name] = t
	}

	// Check for circular extends
	for _, t := range config.Templates {
		if err := checkCircularExtends(t.Name, tmplMap, nil); err != nil {
			return err
		}
	}

	// Resolve extends chains (merge parent rules into children)
	resolved := make(map[string]bool)
	for _, t := range config.Templates {
		if err := resolveExtends(t, tmplMap, resolved, nil); err != nil {
			return err
		}
	}

	// Validate next references in rules
	allRules := append(append(config.Rules, config.PreRules...), config.PostRules...)
	for _, r := range allRules {
		if err := validateNextRefs(r, tmplMap); err != nil {
			return err
		}
	}

	// Validate next references in templates and check for circular next chains
	for _, t := range config.Templates {
		if t.Next != "" {
			if _, ok := tmplMap[t.Next]; !ok {
				return &ParseError{Line: t.Line, Message: fmt.Sprintf("template %q references unknown template %q via next", t.Name, t.Next)}
			}
		}
	}
	for _, t := range config.Templates {
		if err := checkCircularNext(t.Name, tmplMap, nil); err != nil {
			return err
		}
	}

	// Store template index on config for O(1) lookup
	config.TemplateIndex = tmplMap

	return nil
}

func checkCircularExtends(name string, tmplMap map[string]*Template, visited []string) error {
	for _, v := range visited {
		if v == name {
			return &ParseError{
				Line:    tmplMap[name].Line,
				Message: fmt.Sprintf("circular extends detected: %v -> %s", visited, name),
			}
		}
	}

	t, ok := tmplMap[name]
	if !ok || t.Extends == "" {
		return nil
	}

	if _, ok := tmplMap[t.Extends]; !ok {
		return &ParseError{Line: t.Line, Message: fmt.Sprintf("template %q extends unknown template %q", name, t.Extends)}
	}

	return checkCircularExtends(t.Extends, tmplMap, append(visited, name))
}

func resolveExtends(t *Template, tmplMap map[string]*Template, resolved map[string]bool, resolving []string) error {
	if resolved[t.Name] {
		return nil
	}

	// Check for circular resolution
	for _, r := range resolving {
		if r == t.Name {
			return nil // already checked in checkCircularExtends
		}
	}

	if t.Extends != "" {
		parent, ok := tmplMap[t.Extends]
		if !ok {
			return &ParseError{Line: t.Line, Message: fmt.Sprintf("template %q extends unknown template %q", t.Name, t.Extends)}
		}

		// Resolve parent first
		if err := resolveExtends(parent, tmplMap, resolved, append(resolving, t.Name)); err != nil {
			return err
		}

		// Merge parent rules (parent rules come first, child rules override via last-rule-wins)
		t.PipeRules = append(copyRules(parent.PipeRules), t.PipeRules...)
		t.ExecRules = append(copyRules(parent.ExecRules), t.ExecRules...)
		t.ArgsRules = append(copyArgsRules(parent.ArgsRules), t.ArgsRules...)

		if t.Next == "" && parent.Next != "" {
			t.Next = parent.Next
		}
	}

	resolved[t.Name] = true
	return nil
}

func validateNextRefs(r *Rule, tmplMap map[string]*Template) error {
	if r.Next != "" {
		if _, ok := tmplMap[r.Next]; !ok {
			return &ParseError{Line: r.Line, Message: fmt.Sprintf("rule references unknown template %q via next", r.Next)}
		}
	}
	for _, child := range r.PipeRules {
		if err := validateNextRefs(child, tmplMap); err != nil {
			return err
		}
	}
	for _, child := range r.ExecRules {
		if err := validateNextRefs(child, tmplMap); err != nil {
			return err
		}
	}
	return nil
}

func copyRules(rules []*Rule) []*Rule {
	if rules == nil {
		return nil
	}
	out := make([]*Rule, len(rules))
	for i, r := range rules {
		cp := *r
		out[i] = &cp
	}
	return out
}

func copyArgsRules(rules []*ArgsRule) []*ArgsRule {
	if rules == nil {
		return nil
	}
	out := make([]*ArgsRule, len(rules))
	for i, r := range rules {
		cp := *r
		out[i] = &cp
	}
	return out
}

func checkCircularNext(name string, tmplMap map[string]*Template, visited []string) error {
	for _, v := range visited {
		if v == name {
			return &ParseError{
				Line:    tmplMap[name].Line,
				Message: fmt.Sprintf("circular next detected: %v -> %s", visited, name),
			}
		}
	}

	t, ok := tmplMap[name]
	if !ok || t.Next == "" {
		return nil
	}

	return checkCircularNext(t.Next, tmplMap, append(visited, name))
}

// LookupTemplate returns the template with the given name, or nil if not found.
// Uses TemplateIndex for O(1) lookup if available.
func LookupTemplate(config *Config, name string) *Template {
	if config.TemplateIndex != nil {
		return config.TemplateIndex[name]
	}
	// Fallback to linear scan (pre-ResolveTemplates)
	for _, t := range config.Templates {
		if t.Name == name {
			return t
		}
	}
	return nil
}
