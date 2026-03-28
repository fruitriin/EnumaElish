package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fruitriin/ccchain/internal/dsl"
	"github.com/fruitriin/ccchain/internal/eval"
)

// toolInput represents the JSON input from Claude Code's hook system.
type toolInput struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"tool_input"`
}

// bashInput represents the input for a Bash tool call.
type bashInput struct {
	Command string `json:"command"`
}

// fileToolInput represents the input for Read/Edit/Write tool calls.
type fileToolInput struct {
	FilePath string `json:"file_path"`
}

// webFetchInput represents the input for WebFetch tool calls.
type webFetchInput struct {
	URL string `json:"url"`
}

const maxStdinBytes = 1 << 20 // 1MB

func runHookPre(configPath string, defaultAction string) {
	input, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdinBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(1)
	}
	if int64(len(input)) >= maxStdinBytes {
		fmt.Fprintln(os.Stderr, "ccchain: stdin input exceeds 1MB limit (allowing)")
		os.Exit(0)
	}

	var ti toolInput
	if err := json.Unmarshal(input, &ti); err != nil {
		fmt.Fprintf(os.Stderr, "ccchain: invalid hook input JSON (allowing): %v\n", err)
		os.Exit(0)
	}

	cfg, err := dsl.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccchain config error: %v\n", err)
		os.Exit(0)
	}

	if defaultAction != "" {
		if cfg.Settings == nil {
			cfg.Settings = dsl.DefaultSettings()
		}
		cfg.Settings.Fallback = dsl.Action(defaultAction)
	}

	var result *eval.Result

	switch {
	case ti.ToolName == "Bash":
		var bi bashInput
		if err := json.Unmarshal(ti.Input, &bi); err != nil {
			fmt.Fprintf(os.Stderr, "ccchain: invalid Bash input JSON (allowing): %v\n", err)
			os.Exit(0)
		}
		if bi.Command == "" {
			os.Exit(0)
		}
		r, err := eval.Evaluate(bi.Command, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ccchain: parse warning (allowing): %v\n", err)
			os.Exit(0)
		}
		result = r

	case ti.ToolName == "Read" || ti.ToolName == "Edit" || ti.ToolName == "Write":
		var fi fileToolInput
		if err := json.Unmarshal(ti.Input, &fi); err != nil {
			os.Exit(0)
		}
		result = eval.EvaluateTool(ti.ToolName, fi.FilePath, cfg)

	case ti.ToolName == "WebFetch":
		var wf webFetchInput
		if err := json.Unmarshal(ti.Input, &wf); err != nil {
			os.Exit(0)
		}
		result = eval.EvaluateTool(ti.ToolName, wf.URL, cfg)

	case strings.HasPrefix(ti.ToolName, "mcp__"):
		// Best-effort: try to extract file_path, path, or url from MCP input
		mcpArg := extractMCPArg(ti.Input)
		result = eval.EvaluateTool(ti.ToolName, mcpArg, cfg)

	default:
		// Unknown tool — pass through
		os.Exit(0)
	}

	outputResult(result)
}

func outputResult(result *eval.Result) {
	switch result.Action {
	case dsl.ActionAllow:
		os.Exit(0)

	case dsl.ActionDeny:
		msg := result.Message
		if msg == "" {
			msg = "blocked by ccchain"
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(2)

	case dsl.ActionWarn:
		output := map[string]any{
			"decision": "allow",
			"message":  result.Message,
		}
		if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "ccchain: json encode error: %v\n", err)
		}
		os.Exit(0)

	case dsl.ActionAsk:
		output := map[string]any{
			"decision": "ask",
		}
		if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "ccchain: json encode error: %v\n", err)
		}
		os.Exit(0)

	default:
		os.Exit(0)
	}
}

// extractMCPArg attempts to extract a file path or URL from MCP tool input.
func extractMCPArg(input json.RawMessage) string {
	var generic map[string]json.RawMessage
	if json.Unmarshal(input, &generic) != nil {
		return ""
	}
	for _, key := range []string{"file_path", "path", "url", "filePath"} {
		if v, ok := generic[key]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

func runHookPost(configPath string) {
	// PostToolUse hook — currently a pass-through
	// Future: hint actions, turn counting
	os.Exit(0)
}
