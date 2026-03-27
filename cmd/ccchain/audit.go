package main

import (
	"fmt"
	"os"

	"github.com/fruitriin/ccchain/internal/audit"
	"github.com/fruitriin/ccchain/internal/dsl"
)

func runAudit(configPath string) {
	cfg, err := dsl.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	out := audit.Audit(cfg)
	fmt.Print(audit.Format(out))
}
