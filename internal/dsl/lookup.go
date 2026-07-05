package dsl

// CollectTemplatePipeRules collects all pipe rules from a template chain,
// following next: references. visited prevents infinite loops from circular
// next: references.
func CollectTemplatePipeRules(tmpl *Template, config *Config) []*Rule {
	return collectTemplateRules(tmpl, config, make(map[string]bool), pipeRulesOf)
}

// CollectTemplateExecRules collects all exec rules from a template chain,
// following next: references. visited prevents infinite loops from circular
// next: references.
func CollectTemplateExecRules(tmpl *Template, config *Config) []*Rule {
	return collectTemplateRules(tmpl, config, make(map[string]bool), execRulesOf)
}

func pipeRulesOf(tmpl *Template) []*Rule { return tmpl.PipeRules }

func execRulesOf(tmpl *Template) []*Rule { return tmpl.ExecRules }

// collectTemplateRules walks the template chain via next: references and
// collects the rules selected by sel. Circular references are guarded by
// the visited set.
func collectTemplateRules(tmpl *Template, config *Config, visited map[string]bool, sel func(*Template) []*Rule) []*Rule {
	if visited[tmpl.Name] {
		return nil
	}
	visited[tmpl.Name] = true

	var rules []*Rule
	rules = append(rules, sel(tmpl)...)

	if tmpl.Next != "" {
		nextTmpl := LookupTemplate(config, tmpl.Next)
		if nextTmpl != nil {
			rules = append(rules, collectTemplateRules(nextTmpl, config, visited, sel)...)
		}
	}

	return rules
}
