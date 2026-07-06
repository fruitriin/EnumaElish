package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// --- test fixtures -----------------------------------------------------------

const diffTestConfPermissive = `settings:
  fallback: allow

preToolUse

allow ls
`

const diffTestConfStrict = `settings:
  fallback: deny

preToolUse

allow ls
`

// writeDiffTestConf writes content to a temp file and returns its path.
func writeDiffTestConf(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// runDiffMain runs diffMain with captured stdout/stderr.
func runDiffMain(t *testing.T, cmdArgs []string, stdin string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = diffMain("", cmdArgs, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

// --- global CLI parser (C1/C5 regression) ------------------------------------

// TestParseCLIArgs_UnknownFlagRejected verifies that unknown flags after a
// non-whitelisted subcommand are hard errors, not silent pass-throughs.
// Regression test for C1: `ccchain hook pre --defualt-action deny` (typo)
// must fail loudly instead of being ignored, and `ccchain eval --typo "cmd"`
// must not treat --typo as the command to evaluate.
func TestParseCLIArgs_UnknownFlagRejected(t *testing.T) {
	cases := [][]string{
		{"check", "--typo-flag"},
		{"eval", "--typo", "rm -rf /"},
		{"hook", "pre", "--defualt-action", "deny"},
		{"test", "--bogus"},
		{"--typo-before-command", "check"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := parseCLIArgs(args); err == nil {
				t.Errorf("parseCLIArgs(%q) = nil error, want unknown-flag error", args)
			}
		})
	}
}

// TestParseCLIArgs_DiffFlagPassthrough verifies that diff (whitelisted as
// having its own flag parser) receives its flags via cmdArgs.
func TestParseCLIArgs_DiffFlagPassthrough(t *testing.T) {
	c, err := parseCLIArgs([]string{"diff", "a.conf", "b.conf", "--changed-only", "--exit-on-change"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.command != "diff" {
		t.Errorf("command = %q, want diff", c.command)
	}
	want := []string{"a.conf", "b.conf", "--changed-only", "--exit-on-change"}
	if len(c.cmdArgs) != len(want) {
		t.Fatalf("cmdArgs = %q, want %q", c.cmdArgs, want)
	}
	for i := range want {
		if c.cmdArgs[i] != want[i] {
			t.Errorf("cmdArgs[%d] = %q, want %q", i, c.cmdArgs[i], want[i])
		}
	}
}

// TestParseCLIArgs_DiffRejectsGlobalConfig (C5): --config is meaningless for
// diff and must be an explicit error, in both argument orders.
func TestParseCLIArgs_DiffRejectsGlobalConfig(t *testing.T) {
	for _, args := range [][]string{
		{"diff", "a.conf", "b.conf", "--config", "x.conf"},
		{"--config", "x.conf", "diff", "a.conf", "b.conf"},
	} {
		if _, err := parseCLIArgs(args); err == nil {
			t.Errorf("parseCLIArgs(%q) = nil error, want --config rejection", args)
		}
	}
}

// TestParseCLIArgs_HelpAfterDiff (W6): `ccchain diff --help` must be routed to
// the diff-specific usage (showHelp with command already set to diff).
func TestParseCLIArgs_HelpAfterDiff(t *testing.T) {
	c, err := parseCLIArgs([]string{"diff", "--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.showHelp || c.command != "diff" {
		t.Errorf("got showHelp=%v command=%q, want showHelp=true command=diff", c.showHelp, c.command)
	}
}

// --- diffMain: command sources ------------------------------------------------

func TestDiffMain_CommandSourcePriority(t *testing.T) {
	a := writeDiffTestConf(t, "a.conf", diffTestConfPermissive)
	b := writeDiffTestConf(t, "b.conf", diffTestConfStrict)
	cmdFile := writeDiffTestConf(t, "commands.txt", "rm -rf /\n# comment\n\n")

	t.Run("flag --commands", func(t *testing.T) {
		code, out, _ := runDiffMain(t, []string{a, b, "--commands", cmdFile}, "")
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(out, "CHANGED") {
			t.Errorf("expected CHANGED row, got:\n%s", out)
		}
	})

	t.Run("positional file", func(t *testing.T) {
		code, out, _ := runDiffMain(t, []string{a, b, cmdFile}, "")
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(out, "CHANGED") {
			t.Errorf("expected CHANGED row, got:\n%s", out)
		}
	})

	t.Run("stdin fallback", func(t *testing.T) {
		code, out, _ := runDiffMain(t, []string{a, b}, "rm -rf /\n")
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(out, "CHANGED") {
			t.Errorf("expected CHANGED row, got:\n%s", out)
		}
	})

	// S1: both a positional commands file and --commands is ambiguous → error.
	t.Run("both sources is an error", func(t *testing.T) {
		code, _, errOut := runDiffMain(t, []string{a, b, cmdFile, "--commands", cmdFile}, "")
		if code != 1 {
			t.Fatalf("exit = %d, want 1", code)
		}
		if !strings.Contains(errOut, "both --commands and a positional commands file") {
			t.Errorf("unexpected stderr: %s", errOut)
		}
	})
}

// TestDiffMain_PositionalValidation (S1/C2): too many positionals and
// empty/whitespace config paths are usage errors.
func TestDiffMain_PositionalValidation(t *testing.T) {
	a := writeDiffTestConf(t, "a.conf", diffTestConfPermissive)
	b := writeDiffTestConf(t, "b.conf", diffTestConfStrict)

	t.Run("too many positionals", func(t *testing.T) {
		code, _, errOut := runDiffMain(t, []string{a, b, "cmds.txt", "extra"}, "")
		if code != 1 || !strings.Contains(errOut, "too many positional arguments") {
			t.Errorf("exit=%d stderr=%q, want exit 1 with too-many error", code, errOut)
		}
	})

	// C2: an empty config path would fall back to dsl.LoadConfig's default
	// search, silently comparing a config against itself (CI false negative).
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"empty first path", []string{"", b}},
		{"empty second path", []string{a, ""}},
		{"whitespace path", []string{"   ", b}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errOut := runDiffMain(t, tc.args, "ls\n")
			if code != 1 || !strings.Contains(errOut, "non-empty") {
				t.Errorf("exit=%d stderr=%q, want exit 1 with non-empty-path error", code, errOut)
			}
		})
	}

	t.Run("missing positionals", func(t *testing.T) {
		code, _, errOut := runDiffMain(t, []string{a}, "")
		if code != 1 || !strings.Contains(errOut, "requires two config file arguments") {
			t.Errorf("exit=%d stderr=%q, want exit 1 with usage error", code, errOut)
		}
	})
}

// --- diffMain: output ----------------------------------------------------------

func TestDiffMain_ChangedOnly(t *testing.T) {
	a := writeDiffTestConf(t, "a.conf", diffTestConfPermissive)
	b := writeDiffTestConf(t, "b.conf", diffTestConfStrict)

	code, out, _ := runDiffMain(t, []string{a, b, "--changed-only"}, "ls\nrm -rf /\n")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.Contains(out, " same\n") {
		t.Errorf("--changed-only must suppress same rows, got:\n%s", out)
	}
	if !strings.Contains(out, "CHANGED") {
		t.Errorf("expected CHANGED row, got:\n%s", out)
	}
	// Summary still counts the suppressed rows.
	if !strings.Contains(out, "changed=1, same=1, error=0") {
		t.Errorf("unexpected summary:\n%s", out)
	}
}

// TestDiffMain_HeaderBeforeRows (W4): which file is A and which is B must be
// visible before the rows, not only at the very end.
func TestDiffMain_HeaderBeforeRows(t *testing.T) {
	a := writeDiffTestConf(t, "a.conf", diffTestConfPermissive)
	b := writeDiffTestConf(t, "b.conf", diffTestConfStrict)

	code, out, _ := runDiffMain(t, []string{a, b}, "ls\n")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	headerA := strings.Index(out, "Config A: "+a)
	firstRow := strings.Index(out, "ls")
	if headerA < 0 || firstRow < 0 || headerA > firstRow {
		t.Errorf("expected Config A header before first row, got:\n%s", out)
	}
}

// --- diffMain: exit codes (C4) --------------------------------------------------

func TestDiffMain_ExitCodes(t *testing.T) {
	a := writeDiffTestConf(t, "a.conf", diffTestConfPermissive)
	b := writeDiffTestConf(t, "b.conf", diffTestConfStrict)

	t.Run("changed without flag exits 0", func(t *testing.T) {
		code, _, _ := runDiffMain(t, []string{a, b}, "rm -rf /\n")
		if code != 0 {
			t.Errorf("exit = %d, want 0", code)
		}
	})

	t.Run("changed with --exit-on-change exits 2", func(t *testing.T) {
		code, _, _ := runDiffMain(t, []string{a, b, "--exit-on-change"}, "rm -rf /\n")
		if code != 2 {
			t.Errorf("exit = %d, want 2", code)
		}
	})

	t.Run("same with --exit-on-change exits 0", func(t *testing.T) {
		code, _, _ := runDiffMain(t, []string{a, b, "--exit-on-change"}, "ls\n")
		if code != 0 {
			t.Errorf("exit = %d, want 0", code)
		}
	})

	t.Run("evaluation error exits 3", func(t *testing.T) {
		code, out, _ := runDiffMain(t, []string{a, b}, "echo \"unclosed\n")
		if code != 3 {
			t.Errorf("exit = %d, want 3", code)
		}
		if !strings.Contains(out, "ERROR") {
			t.Errorf("expected ERROR row, got:\n%s", out)
		}
	})

	// C4: a single broken command must not masquerade as "rule changed";
	// error (3) takes precedence over changed (2).
	t.Run("error takes precedence over changed", func(t *testing.T) {
		code, _, _ := runDiffMain(t, []string{a, b, "--exit-on-change"}, "rm -rf /\necho \"unclosed\n")
		if code != 3 {
			t.Errorf("exit = %d, want 3", code)
		}
	})
}

// --- diffMain: control character sanitization (C3) ------------------------------

func TestDiffMain_ControlCharsEscaped(t *testing.T) {
	a := writeDiffTestConf(t, "a.conf", diffTestConfPermissive)
	b := writeDiffTestConf(t, "b.conf", diffTestConfStrict)

	// \r would let a row overwrite itself; ESC could start an ANSI sequence
	// hiding CHANGED rows from a human reviewing terminal output.
	code, out, _ := runDiffMain(t, []string{a, b}, "ls \r\x1b[2K --hidden\n")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.ContainsAny(out, "\r\x1b") {
		t.Errorf("raw control characters leaked into output: %q", out)
	}
	if !strings.Contains(out, `\x0d`) || !strings.Contains(out, `\x1b`) {
		t.Errorf("expected escaped control characters in output, got: %q", out)
	}
}

func TestSanitizeControlChars(t *testing.T) {
	cases := map[string]string{
		"plain command":  "plain command",
		"tab\there":      `tab\x09here`,
		"cr\rlf\n":       `cr\x0dlf\x0a`,
		"esc\x1b[31mred": `esc\x1b[31mred`,
		"del\x7fchar":    `del\x7fchar`,
		"nul\x00byte":    `nul\x00byte`,
		"日本語 そのまま":       "日本語 そのまま",
	}
	for in, want := range cases {
		if got := sanitizeControlChars(in); got != want {
			t.Errorf("sanitizeControlChars(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- shared helpers (W2/W3) -----------------------------------------------------

// TestLoadCommandsFromReader_LongLine (W2): a single line above bufio's 64KB
// default must not abort the whole scan.
func TestLoadCommandsFromReader_LongLine(t *testing.T) {
	long := "echo " + strings.Repeat("x", 100*1024)
	input := long + "\nls\n"
	cmds, err := loadCommandsFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("got %d commands, want 2", len(cmds))
	}
	if cmds[0] != long || cmds[1] != "ls" {
		t.Errorf("commands not preserved (lens: %d, %d)", len(cmds[0]), len(cmds[1]))
	}
}

// TestTruncateStr_UTF8Safe (W3): truncation must never split a multi-byte rune.
func TestTruncateStr_UTF8Safe(t *testing.T) {
	s := strings.Repeat("あ", 40) // 120 bytes
	for n := 3; n < 20; n++ {
		got := truncateStr(s, n)
		if !utf8.ValidString(got) {
			t.Errorf("truncateStr(%d) produced invalid UTF-8: %q", n, got)
		}
		if !strings.HasSuffix(got, "...") {
			t.Errorf("truncateStr(%d) missing ellipsis: %q", n, got)
		}
	}
	// Short strings pass through untouched.
	if got := truncateStr("short", 60); got != "short" {
		t.Errorf("truncateStr(short) = %q", got)
	}
}
