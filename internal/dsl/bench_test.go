package dsl

import (
	"os"
	"strings"
	"testing"
)

func BenchmarkLexer(b *testing.B) {
	data, err := os.ReadFile("../../testdata/dsl/templates.conf")
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	input := string(data)

	b.ResetTimer()
	for b.Loop() {
		_, err := Lex(strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParser(b *testing.B) {
	data, err := os.ReadFile("../../testdata/dsl/templates.conf")
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	input := string(data)

	b.ResetTimer()
	for b.Loop() {
		_, err := Parse(strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTemplateResolve(b *testing.B) {
	data, err := os.ReadFile("../../testdata/dsl/templates.conf")
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	input := string(data)

	b.ResetTimer()
	for b.Loop() {
		cfg, err := Parse(strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
		if err := ResolveTemplates(cfg); err != nil {
			b.Fatal(err)
		}
	}
}
