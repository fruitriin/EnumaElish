package dsl

import (
	"os"
	"strings"
	"testing"
)

func TestResolveTemplatesExtends(t *testing.T) {
	f, err := os.Open("../../testdata/dsl/templates.conf")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	cfg, err := Parse(f)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err = ResolveTemplates(cfg)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}

	// bulkExec extends safeRead, which has next: primitive
	bulk := LookupTemplate(cfg, "bulkExec")
	if bulk == nil {
		t.Fatal("bulkExec template not found")
	}

	// After extends resolution, bulkExec should have safeRead's next
	assertEqual(t, "bulkExec.next", bulk.Next, "primitive")
}

func TestResolveCircularExtends(t *testing.T) {
	f, err := os.Open("../../testdata/dsl/error_circular.conf")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	cfg, err := Parse(f)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err = ResolveTemplates(cfg)
	if err == nil {
		t.Fatal("expected circular reference error, got nil")
	}

	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("expected error about circular reference, got: %v", err)
	}
}

func TestResolveUnknownTemplate(t *testing.T) {
	f, err := os.Open("../../testdata/dsl/error_unknown_template.conf")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	cfg, err := Parse(f)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err = ResolveTemplates(cfg)
	if err == nil {
		t.Fatal("expected unknown template error, got nil")
	}

	if !strings.Contains(err.Error(), "unknown template") {
		t.Errorf("expected error about unknown template, got: %v", err)
	}
}

func TestResolveDuplicateTemplate(t *testing.T) {
	input := `template foo
  |,>>
    allow cat

template foo
  |,>>
    allow dog`

	cfg, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err = ResolveTemplates(cfg)
	if err == nil {
		t.Fatal("expected duplicate template error, got nil")
	}

	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected error about duplicate template, got: %v", err)
	}
}
