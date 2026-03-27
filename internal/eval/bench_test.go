package eval

import (
	"strings"
	"testing"

	"github.com/fruitriin/ccchain/internal/dsl"
)

var benchConfig *dsl.Config

func init() {
	input := `
template primitive
  |,>>
    allow cat, echo, head, tail, wc, sort, uniq

template safeRead
  next: primitive
  |,>>
    allow grep, awk, sed

template bulkExec
  extends: safeRead
  |,>>
    deny rm  "don't pipe into destructive commands"
  exec:
    deny rm  "expand to tempfile first"
    allow cp, mv, touch

allow ls
  next: primitive

allow find
  next: bulkExec

allow xargs
  next: bulkExec

allow grep
  next: safeRead

deny rm

allow curl
  |
    deny bash  "curl | bash is not allowed"
    deny sh    "curl | sh is not allowed"

deny eval  "eval is not allowed"

settings:
  fallback: ask
`
	cfg, err := dsl.Parse(strings.NewReader(input))
	if err != nil {
		panic(err)
	}
	if err := dsl.ResolveTemplates(cfg); err != nil {
		panic(err)
	}
	benchConfig = cfg
}

func BenchmarkEvaluate(b *testing.B) {
	for b.Loop() {
		_, err := Evaluate("find . | grep foo | head -5", benchConfig)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEndToEnd(b *testing.B) {
	for b.Loop() {
		_, err := Evaluate("find . -name '*.log' | grep error | head -20 && echo done; ls -la | wc -l", benchConfig)
		if err != nil {
			b.Fatal(err)
		}
	}
}
