package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fruitriin/ccchain/internal/semantics"
)

type projectType struct {
	Name       string
	Indicators []string
	Commands   []string // command names from semantics table
	Extra      []string // additional rules not in semantics table
}

var projectTypes = []projectType{
	{
		Name:       "Go",
		Indicators: []string{"go.mod", "go.sum"},
		Commands:   []string{"go"},
	},
	{
		Name:       "Node.js",
		Indicators: []string{"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "bun.lockb"},
		Commands:   []string{"npm", "yarn", "pnpm", "npx"},
		Extra:      []string{"# ask node  # node -e can execute arbitrary code"},
	},
	{
		Name:       "Rust",
		Indicators: []string{"Cargo.toml", "Cargo.lock"},
		Commands:   []string{"cargo"},
		Extra:      []string{"allow rustc"},
	},
	{
		Name:       "Python",
		Indicators: []string{"pyproject.toml", "setup.py", "requirements.txt", "Pipfile", "uv.lock"},
		Commands:   []string{"pip", "uv"},
		Extra:      []string{"# ask python3  # python3 -c can execute arbitrary code"},
	},
	{
		Name:       "Ruby",
		Indicators: []string{"Gemfile", "Gemfile.lock"},
		Extra:      []string{"allow bundle", "allow rake"},
	},
	{
		Name:       "Docker",
		Indicators: []string{"Dockerfile", "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"},
		Commands:   []string{"docker"},
		Extra:      []string{"ask docker-compose"},
	},
	{
		Name:       "Kubernetes",
		Indicators: []string{"k8s/", "kubernetes/", "kustomization.yaml", "helmfile.yaml"},
		Commands:   []string{"kubectl"},
		Extra:      []string{"ask helm"},
	},
	{
		Name:       "Terraform",
		Indicators: []string{"main.tf", "terraform.tfstate", ".terraform/"},
		Commands:   []string{"terraform"},
	},
	{
		Name:       "Make",
		Indicators: []string{"Makefile", "GNUmakefile", "makefile"},
		Extra:      []string{"allow make"},
	},
}

func runDetect() {
	var detected []projectType

	for _, pt := range projectTypes {
		for _, ind := range pt.Indicators {
			if fileExists(ind) {
				detected = append(detected, pt)
				break
			}
		}
	}

	if len(detected) == 0 {
		fmt.Println("# No known project types detected.")
		fmt.Println("# Use 'ccchain generate-rules' for a full semantics-based ruleset.")
		return
	}

	for _, pt := range detected {
		fmt.Printf("# Detected: %s\n", pt.Name)
	}
	fmt.Println()
	fmt.Println("# Suggested rules for .ccchain.conf:")
	fmt.Println()

	seen := make(map[string]bool)
	for _, pt := range detected {
		for _, cmd := range pt.Commands {
			if seen[cmd] {
				continue
			}
			seen[cmd] = true

			sem, ok := semantics.Table[cmd]
			if !ok {
				fmt.Printf("allow %s\n\n", cmd)
				continue
			}

			action := sem.DefaultAction
			if action == "" {
				action = "ask"
			}
			fmt.Printf("%s %s\n", action, cmd)

			hasArgs := len(sem.SafeSubcommands) > 0 || len(sem.DangerousSubcommands) > 0 || len(sem.DestructiveArgs) > 0
			if hasArgs {
				fmt.Println("  args:")
				if len(sem.SafeSubcommands) > 0 {
					fmt.Printf("    ^(%s)\\b: allow\n", strings.Join(sem.SafeSubcommands, "|"))
				}
				if len(sem.DangerousSubcommands) > 0 {
					msg := cmd + " subcommand can have significant effects"
					fmt.Printf("    ^(%s)\\b: ask  \"%s\"\n", strings.Join(sem.DangerousSubcommands, "|"), msg)
				}
				if len(sem.DestructiveArgs) > 0 {
					msg := cmd + " with these flags can be destructive"
					fmt.Printf("    %s: ask  \"%s\"\n", strings.Join(sem.DestructiveArgs, "|"), msg)
				}
			}
			fmt.Println()
		}

		for _, extra := range pt.Extra {
			if !seen[extra] {
				fmt.Println(extra)
			}
		}
	}

	fmt.Println()
	fmt.Println("# Review these suggestions, then append to .ccchain.conf")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
