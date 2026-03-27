package dsl

import (
	"strings"
	"testing"
)

func TestLexBasicRules(t *testing.T) {
	input := `allow find
  |,>>
    deny rm  "don't combine"
deny rm`

	lexer, err := Lex(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	lines := lexer.Lines()
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	// Line 1: allow find
	assertToken(t, lines[0].Tokens[0], TokenAction, "allow")
	assertToken(t, lines[0].Tokens[1], TokenIdent, "find")
	assertEqual(t, "line 1 indent", lines[0].Indent, 0)

	// Line 2: |,>>
	assertToken(t, lines[1].Tokens[0], TokenContext, "|,>>")
	assertEqual(t, "line 2 indent", lines[1].Indent, 2)

	// Line 3: deny rm "don't combine"
	assertToken(t, lines[2].Tokens[0], TokenAction, "deny")
	assertToken(t, lines[2].Tokens[1], TokenIdent, "rm")
	assertToken(t, lines[2].Tokens[2], TokenString, "don't combine")
	assertEqual(t, "line 3 indent", lines[2].Indent, 4)

	// Line 4: deny rm
	assertToken(t, lines[3].Tokens[0], TokenAction, "deny")
	assertToken(t, lines[3].Tokens[1], TokenIdent, "rm")
	assertEqual(t, "line 4 indent", lines[3].Indent, 0)
}

func TestLexComments(t *testing.T) {
	input := `# full line comment
allow ls  # inline comment
# another comment
deny rm`

	lexer, err := Lex(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	lines := lexer.Lines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (comments skipped), got %d", len(lines))
	}
}

func TestLexKeywords(t *testing.T) {
	input := `template primitive
  extends: safeRead
  next: primitive
settings:
  max_context_depth: 2
  fallback: ask`

	lexer, err := Lex(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	lines := lexer.Lines()
	assertToken(t, lines[0].Tokens[0], TokenKeyword, "template")
	assertToken(t, lines[0].Tokens[1], TokenIdent, "primitive")
	assertToken(t, lines[1].Tokens[0], TokenKeyword, "extends")
	assertToken(t, lines[2].Tokens[0], TokenKeyword, "next")
	assertToken(t, lines[3].Tokens[0], TokenKeyword, "settings")
}

func TestLexMultipleCommands(t *testing.T) {
	input := `allow cat, echo, head, tail`

	lexer, err := Lex(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	lines := lexer.Lines()
	tokens := lines[0].Tokens

	// allow, cat, ',', echo, ',', head, ',', tail
	identCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenIdent {
			identCount++
		}
	}
	if identCount != 4 {
		t.Errorf("expected 4 idents, got %d", identCount)
	}
}

func TestLexContextVariants(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"|", "|"},
		{">>", ">>"},
		{"|,>>", "|,>>"},
	}

	for _, tt := range tests {
		lexer, err := Lex(strings.NewReader(tt.input))
		if err != nil {
			t.Fatalf("Lex error for %q: %v", tt.input, err)
		}
		lines := lexer.Lines()
		if len(lines) != 1 {
			t.Fatalf("expected 1 line for %q, got %d", tt.input, len(lines))
		}
		assertToken(t, lines[0].Tokens[0], TokenContext, tt.expected)
	}
}

func assertToken(t *testing.T, got Token, expectedType TokenType, expectedValue string) {
	t.Helper()
	if got.Type != expectedType {
		t.Errorf("expected token type %s, got %s (value: %q)", expectedType, got.Type, got.Value)
	}
	if got.Value != expectedValue {
		t.Errorf("expected token value %q, got %q", expectedValue, got.Value)
	}
}

func assertEqual[T comparable](t *testing.T, name string, got, expected T) {
	t.Helper()
	if got != expected {
		t.Errorf("%s: expected %v, got %v", name, expected, got)
	}
}
