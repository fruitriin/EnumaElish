package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fruitriin/ccchain/internal/dsl"
	"github.com/fruitriin/ccchain/internal/eval"
)

func runEval(configPath string, cmdArgs []string) {
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "error: ccchain eval requires a command string")
		fmt.Fprintln(os.Stderr, "usage: ccchain eval \"command string\"")
		os.Exit(1)
	}

	cmdStr := cmdArgs[0]

	cfg, err := dsl.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	result, err := eval.Evaluate(cmdStr, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "evaluation error: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "json encode error: %v\n", err)
		os.Exit(1)
	}
}
