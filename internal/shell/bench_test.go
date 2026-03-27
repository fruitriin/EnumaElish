package shell

import (
	"testing"
)

func BenchmarkShellParse(b *testing.B) {
	cmd := "find . -name '*.log' | grep error | head -20 && echo done; ls -la | wc -l"
	for b.Loop() {
		_, err := ParseCommand(cmd)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTopologyBuild(b *testing.B) {
	cmd := "find . -name '*.log' | grep error | head -20 && echo done; ls -la | wc -l"
	for b.Loop() {
		_, err := BuildTopology(cmd)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNestedParse(b *testing.B) {
	cmd := `bash -c "find . | grep foo | head -5"`
	for b.Loop() {
		_, err := BuildTopology(cmd)
		if err != nil {
			b.Fatal(err)
		}
	}
}
