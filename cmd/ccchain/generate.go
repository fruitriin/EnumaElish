package main

import (
	"fmt"

	"github.com/fruitriin/ccchain/internal/semantics"
)

func runGenerateRules() {
	fmt.Print(semantics.GenerateRules())
}
