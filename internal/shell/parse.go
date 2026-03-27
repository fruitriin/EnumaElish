// Package shell implements shell command structural analysis using mvdan.cc/sh.
package shell

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// ParseCommand parses a shell command string into an mvdan.cc/sh AST.
// Uses bash mode (syntax.LangBash) to support bash extensions like <().
func ParseCommand(cmd string) (*syntax.File, error) {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	return parser.Parse(strings.NewReader(cmd), "")
}
