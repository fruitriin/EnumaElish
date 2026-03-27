package dsl

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ParseError represents a DSL parsing error with location information.
type ParseError struct {
	Line    int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Message)
}

// Parse parses DSL input and returns a Config.
func Parse(r io.Reader) (*Config, error) {
	lexer, err := Lex(r)
	if err != nil {
		return nil, fmt.Errorf("lexer error: %w", err)
	}

	p := &parser{
		lines: lexer.Lines(),
		pos:   0,
	}

	return p.parseConfig()
}

type parser struct {
	lines []Line
	pos   int
}

func (p *parser) current() *Line {
	if p.pos >= len(p.lines) {
		return nil
	}
	return &p.lines[p.pos]
}

func (p *parser) advance() {
	p.pos++
}

func (p *parser) parseConfig() (*Config, error) {
	config := &Config{
		Settings: DefaultSettings(),
	}

	for p.current() != nil {
		line := p.current()
		if line.Indent != 0 {
			return nil, &ParseError{Line: line.LineNo, Message: "unexpected indentation at top level"}
		}

		if len(line.Tokens) == 0 {
			p.advance()
			continue
		}

		tok := line.Tokens[0]

		switch {
		case tok.Type == TokenKeyword && tok.Value == "template":
			tmpl, err := p.parseTemplate()
			if err != nil {
				return nil, err
			}
			config.Templates = append(config.Templates, tmpl)

		case tok.Type == TokenKeyword && tok.Value == "preToolUse":
			p.advance()
			rules, err := p.parseRulesBlock(0)
			if err != nil {
				return nil, err
			}
			config.PreRules = append(config.PreRules, rules...)

		case tok.Type == TokenKeyword && tok.Value == "postToolUse":
			p.advance()
			rules, err := p.parseRulesBlock(0)
			if err != nil {
				return nil, err
			}
			config.PostRules = append(config.PostRules, rules...)

		case tok.Type == TokenKeyword && tok.Value == "settings":
			settings, err := p.parseSettings()
			if err != nil {
				return nil, err
			}
			config.Settings = settings

		case tok.Type == TokenAction:
			rule, err := p.parseRule(0)
			if err != nil {
				return nil, err
			}
			config.Rules = append(config.Rules, rule)

		default:
			return nil, &ParseError{Line: line.LineNo, Message: fmt.Sprintf("unexpected token: %s %q", tok.Type, tok.Value)}
		}
	}

	return config, nil
}

func (p *parser) parseTemplate() (*Template, error) {
	line := p.current()
	if len(line.Tokens) < 2 {
		return nil, &ParseError{Line: line.LineNo, Message: "template requires a name"}
	}

	tmpl := &Template{
		Name: line.Tokens[1].Value,
		Line: line.LineNo,
	}

	baseIndent := line.Indent
	p.advance()

	for p.current() != nil && p.current().Indent > baseIndent {
		childLine := p.current()
		tok := childLine.Tokens[0]

		switch {
		case tok.Type == TokenKeyword && tok.Value == "extends":
			if len(childLine.Tokens) < 3 { // extends : name
				// try extends: name (colon attached)
				val, err := p.parseKeyValue(childLine)
				if err != nil {
					return nil, err
				}
				tmpl.Extends = val
			} else {
				tmpl.Extends = childLine.Tokens[2].Value
			}
			p.advance()

		case tok.Type == TokenKeyword && tok.Value == "next":
			val, err := p.parseKeyValue(childLine)
			if err != nil {
				return nil, err
			}
			tmpl.Next = val
			p.advance()

		case tok.Type == TokenContext:
			p.advance()
			rules, err := p.parseRulesBlock(childLine.Indent)
			if err != nil {
				return nil, err
			}
			tmpl.PipeRules = append(tmpl.PipeRules, rules...)

		case tok.Type == TokenKeyword && tok.Value == "exec":
			p.advance()
			rules, err := p.parseRulesBlock(childLine.Indent)
			if err != nil {
				return nil, err
			}
			tmpl.ExecRules = append(tmpl.ExecRules, rules...)

		case tok.Type == TokenKeyword && tok.Value == "args":
			p.advance()
			argsRules, err := p.parseArgsBlock(childLine.Indent)
			if err != nil {
				return nil, err
			}
			tmpl.ArgsRules = append(tmpl.ArgsRules, argsRules...)

		default:
			return nil, &ParseError{Line: childLine.LineNo, Message: fmt.Sprintf("unexpected token in template: %q", tok.Value)}
		}
	}

	return tmpl, nil
}

func (p *parser) parseRule(parentIndent int) (*Rule, error) {
	line := p.current()
	if len(line.Tokens) == 0 || line.Tokens[0].Type != TokenAction {
		return nil, &ParseError{Line: line.LineNo, Message: "expected action (allow/deny/warn/ask/hint)"}
	}

	rule := &Rule{
		Action: Action(line.Tokens[0].Value),
		Line:   line.LineNo,
	}

	// Parse commands and optional message
	for i := 1; i < len(line.Tokens); i++ {
		tok := line.Tokens[i]
		switch tok.Type {
		case TokenIdent:
			rule.Commands = append(rule.Commands, tok.Value)
		case TokenString:
			rule.Message = tok.Value
		case TokenComma:
			// skip commas between commands
		default:
			// ignore unexpected tokens in rule line
		}
	}

	baseIndent := line.Indent
	p.advance()

	// Parse child blocks
	for p.current() != nil && p.current().Indent > baseIndent {
		childLine := p.current()
		tok := childLine.Tokens[0]

		switch {
		case tok.Type == TokenContext:
			p.advance()
			rules, err := p.parseRulesBlock(childLine.Indent)
			if err != nil {
				return nil, err
			}
			rule.PipeRules = append(rule.PipeRules, rules...)

		case tok.Type == TokenKeyword && tok.Value == "exec":
			p.advance()
			rules, err := p.parseRulesBlock(childLine.Indent)
			if err != nil {
				return nil, err
			}
			rule.ExecRules = append(rule.ExecRules, rules...)

		case tok.Type == TokenKeyword && tok.Value == "args":
			p.advance()
			argsRules, err := p.parseArgsBlock(childLine.Indent)
			if err != nil {
				return nil, err
			}
			rule.ArgsRules = append(rule.ArgsRules, argsRules...)

		case tok.Type == TokenKeyword && tok.Value == "next":
			val, err := p.parseKeyValue(childLine)
			if err != nil {
				return nil, err
			}
			rule.Next = val
			p.advance()

		case tok.Type == TokenKeyword && tok.Value == "mode":
			val, err := p.parseKeyValue(childLine)
			if err != nil {
				return nil, err
			}
			rule.Mode = val
			// mode: is deprecated — parsed for backward compatibility but has no effect
			fmt.Fprintf(os.Stderr, "ccchain: warning: line %d: mode: property is deprecated and has no effect. Use 'warn' or 'hint' actions directly.\n", childLine.LineNo)
			p.advance()

		case tok.Type == TokenKeyword && tok.Value == "message":
			val, err := p.parseKeyValue(childLine)
			if err != nil {
				return nil, err
			}
			rule.Message = val
			p.advance()

		default:
			return nil, &ParseError{Line: childLine.LineNo, Message: fmt.Sprintf("unexpected token in rule: %q", tok.Value)}
		}
	}

	return rule, nil
}

func (p *parser) parseRulesBlock(parentIndent int) ([]*Rule, error) {
	var rules []*Rule

	for p.current() != nil && p.current().Indent > parentIndent {
		line := p.current()
		if line.Tokens[0].Type == TokenAction {
			rule, err := p.parseRule(parentIndent)
			if err != nil {
				return nil, err
			}
			rules = append(rules, rule)
		} else {
			// Not a rule — stop parsing this block
			break
		}
	}

	return rules, nil
}

func (p *parser) parseArgsBlock(parentIndent int) ([]*ArgsRule, error) {
	var rules []*ArgsRule

	for p.current() != nil && p.current().Indent > parentIndent {
		line := p.current()
		// Expect: pattern : action [message]
		// Everything before ':' is the pattern, everything after is action [message]
		ar := &ArgsRule{Line: line.LineNo}

		colonIdx := -1
		for i, tok := range line.Tokens {
			if tok.Type == TokenColon {
				colonIdx = i
				break
			}
		}

		if colonIdx >= 0 {
			// Build pattern from tokens before colon
			var patternParts []string
			for i := 0; i < colonIdx; i++ {
				patternParts = append(patternParts, line.Tokens[i].Value)
			}
			ar.Pattern = strings.Join(patternParts, " ")

			// Parse action and message after colon
			for i := colonIdx + 1; i < len(line.Tokens); i++ {
				tok := line.Tokens[i]
				if tok.Type == TokenAction || (tok.Type == TokenIdent && actions[tok.Value]) {
					ar.Action = Action(tok.Value)
				} else if tok.Type == TokenString {
					ar.Message = tok.Value
				}
			}
		}

		if ar.Pattern != "" && ar.Action != "" {
			rules = append(rules, ar)
		}
		p.advance()
	}

	return rules, nil
}

func (p *parser) parseSettings() (*Settings, error) {
	line := p.current()
	settings := DefaultSettings()
	settings.Line = line.LineNo
	baseIndent := line.Indent
	p.advance()

	for p.current() != nil && p.current().Indent > baseIndent {
		childLine := p.current()
		key := childLine.Tokens[0].Value

		val, err := p.parseKeyValue(childLine)
		if err != nil {
			return nil, err
		}

		switch key {
		case "max_context_depth":
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, &ParseError{Line: childLine.LineNo, Message: fmt.Sprintf("invalid number for max_context_depth: %q", val)}
			}
			settings.MaxContextDepth = n
		case "max_rules_per_cmd":
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, &ParseError{Line: childLine.LineNo, Message: fmt.Sprintf("invalid number for max_rules_per_cmd: %q", val)}
			}
			settings.MaxRulesPerCmd = n
		case "fallback":
			if !IsValidAction(val) {
				return nil, &ParseError{Line: childLine.LineNo, Message: fmt.Sprintf("invalid fallback action: %q", val)}
			}
			settings.Fallback = Action(val)
		default:
			return nil, &ParseError{Line: childLine.LineNo, Message: fmt.Sprintf("unknown setting: %q", key)}
		}

		p.advance()
	}

	return settings, nil
}

// parseKeyValue extracts the value from a "key: value" or "key : value" line.
func (p *parser) parseKeyValue(line *Line) (string, error) {
	tokens := line.Tokens
	// Find the colon and return everything after it
	for i, tok := range tokens {
		if tok.Type == TokenColon {
			if i+1 < len(tokens) {
				return tokens[i+1].Value, nil
			}
			return "", &ParseError{Line: line.LineNo, Message: "expected value after ':'"}
		}
	}
	// No colon found — try key value (without colon)
	if len(tokens) >= 2 {
		return tokens[1].Value, nil
	}
	return "", &ParseError{Line: line.LineNo, Message: "expected key: value"}
}
