package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

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

func runHookPre(configPath string) {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(1)
	}

	var ti toolInput
	if err := json.Unmarshal(input, &ti); err != nil {
		fmt.Fprintf(os.Stderr, "ccchain: invalid hook input JSON (allowing): %v\n", err)
		os.Exit(0)
	}

	// Only process Bash tool calls
	if ti.ToolName != "Bash" {
		os.Exit(0)
	}

	var bi bashInput
	if err := json.Unmarshal(ti.Input, &bi); err != nil {
		fmt.Fprintf(os.Stderr, "ccchain: invalid Bash input JSON (allowing): %v\n", err)
		os.Exit(0)
	}

	if bi.Command == "" {
		os.Exit(0)
	}

	cfg, err := dsl.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccchain config error: %v\n", err)
		// Config error — don't block, just warn
		os.Exit(0)
	}

	result, err := eval.Evaluate(bi.Command, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccchain: parse warning (allowing): %v\n", err)
		os.Exit(0)
	}

	switch result.Action {
	case dsl.ActionAllow:
		os.Exit(0)

	case dsl.ActionDeny:
		msg := result.Message
		if msg == "" {
			msg = "command blocked by ccchain"
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

func runHookPost(configPath string) {
	// PostToolUse hook — currently a pass-through
	// Future: hint actions, turn counting
	os.Exit(0)
}
