package dsl

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// TokenType represents the type of a lexer token.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenAction        // allow, deny, warn, ask, hint
	TokenKeyword       // template, extends, next, preToolUse, postToolUse, settings, exec, args, mode, message
	TokenContext       // |,>>  or |  or >>
	TokenIdent         // command name, template name, etc.
	TokenString        // "quoted string"
	TokenColon         // :
	TokenComma         // ,
	TokenNumber        // integer
)

// Token represents a single lexer token.
type Token struct {
	Type   TokenType
	Value  string
	Line   int
	Indent int // indentation level (number of leading spaces/tabs)
}

func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenAction:
		return "Action"
	case TokenKeyword:
		return "Keyword"
	case TokenContext:
		return "Context"
	case TokenIdent:
		return "Ident"
	case TokenString:
		return "String"
	case TokenColon:
		return "Colon"
	case TokenComma:
		return "Comma"
	case TokenNumber:
		return "Number"
	default:
		return "Unknown"
	}
}

var actions = map[string]bool{
	"allow": true,
	"deny":  true,
	"warn":  true,
	"ask":   true,
	"hint":  true,
}

var keywords = map[string]bool{
	"template":    true,
	"extends":     true,
	"next":        true,
	"preToolUse":  true,
	"postToolUse": true,
	"settings":    true,
	"exec":        true,
	"args":        true,
	"mode":        true,
	"message":     true,
}

// Line represents a tokenized line with its indentation.
type Line struct {
	Tokens []Token
	Indent int
	LineNo int
}

// Lexer tokenizes DSL input.
type Lexer struct {
	lines []Line
}

// Lex tokenizes the input and returns a Lexer.
func Lex(r io.Reader) (*Lexer, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB max line length
	var lines []Line
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()

		// Skip empty lines and comments
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := countIndent(raw)
		tokens, err := tokenizeLine(trimmed, lineNo, indent)
		if err != nil {
			return nil, err
		}
		if len(tokens) > 0 {
			lines = append(lines, Line{
				Tokens: tokens,
				Indent: indent,
				LineNo: lineNo,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Lexer{lines: lines}, nil
}

// Lines returns the tokenized lines.
func (l *Lexer) Lines() []Line {
	return l.lines
}

func countIndent(s string) int {
	count := 0
	for _, ch := range s {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 4 // treat tab as 4 spaces
		} else {
			break
		}
	}
	return count
}

func tokenizeLine(s string, lineNo, indent int) ([]Token, error) {
	var tokens []Token
	i := 0

	for i < len(s) {
		// Skip whitespace
		if s[i] == ' ' || s[i] == '\t' {
			i++
			continue
		}

		// Context: |,>> or | or >>
		if s[i] == '|' || (s[i] == '>' && i+1 < len(s) && s[i+1] == '>') {
			ctx := parseContext(s, &i)
			tokens = append(tokens, Token{Type: TokenContext, Value: ctx, Line: lineNo, Indent: indent})
			continue
		}

		// Quoted string
		if s[i] == '"' {
			str, err := parseString(s, &i, lineNo)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, Token{Type: TokenString, Value: str, Line: lineNo, Indent: indent})
			continue
		}

		// Colon
		if s[i] == ':' {
			tokens = append(tokens, Token{Type: TokenColon, Value: ":", Line: lineNo, Indent: indent})
			i++
			continue
		}

		// Comma
		if s[i] == ',' {
			tokens = append(tokens, Token{Type: TokenComma, Value: ",", Line: lineNo, Indent: indent})
			i++
			continue
		}

		// Comment (inline)
		if s[i] == '#' {
			break
		}

		// Word (ident, action, keyword, or number)
		word := parseWord(s, &i)
		if word == "" {
			i++
			continue
		}

		if actions[word] {
			tokens = append(tokens, Token{Type: TokenAction, Value: word, Line: lineNo, Indent: indent})
		} else if keywords[word] {
			tokens = append(tokens, Token{Type: TokenKeyword, Value: word, Line: lineNo, Indent: indent})
		} else if isNumber(word) {
			tokens = append(tokens, Token{Type: TokenNumber, Value: word, Line: lineNo, Indent: indent})
		} else {
			tokens = append(tokens, Token{Type: TokenIdent, Value: word, Line: lineNo, Indent: indent})
		}
	}

	return tokens, nil
}

func parseContext(s string, i *int) string {
	var parts []string
	for *i < len(s) {
		if s[*i] == '|' {
			parts = append(parts, "|")
			*i++
		} else if s[*i] == '>' && *i+1 < len(s) && s[*i+1] == '>' {
			parts = append(parts, ">>")
			*i += 2
		} else if s[*i] == ',' {
			*i++
		} else {
			break
		}
	}
	return strings.Join(parts, ",")
}

func parseString(s string, i *int, lineNo int) (string, error) {
	*i++ // skip opening "
	var b strings.Builder
	for *i < len(s) {
		if s[*i] == '\\' && *i+1 < len(s) {
			*i++
			b.WriteByte(s[*i])
			*i++
			continue
		}
		if s[*i] == '"' {
			*i++
			return b.String(), nil
		}
		b.WriteByte(s[*i])
		*i++
	}
	return "", fmt.Errorf("line %d: unterminated string", lineNo)
}

func parseWord(s string, i *int) string {
	start := *i
	for *i < len(s) && s[*i] != ' ' && s[*i] != '\t' && s[*i] != ':' && s[*i] != ',' && s[*i] != '#' && s[*i] != '"' {
		*i++
	}
	return s[start:*i]
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
