package dsl

import (
	"fmt"
	"os"
	"path/filepath"
)

// LoadConfig loads and parses ccchain configuration from the default search paths.
// Search order:
// 1. explicit configPath (if non-empty)
// 2. .ccchain.conf (project root)
// 3. .ccchain.local.conf (local override)
// 4. $CLAUDE_CONFIG_DIR/ccchain.conf
// 5. ~/.claude/ccchain.conf (fallback)
//
// Files are merged in order: later files override earlier ones via last-rule-wins.
func LoadConfig(configPath string) (*Config, error) {
	if configPath != "" {
		return loadAndParse(configPath)
	}

	paths := searchPaths()
	var configs []*Config

	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			continue
		}
		cfg, err := loadAndParse(p)
		if err != nil {
			return nil, fmt.Errorf("error in %s: %w", p, err)
		}
		configs = append(configs, cfg)
	}

	if len(configs) == 0 {
		return &Config{Settings: DefaultSettings()}, nil
	}

	// Merge configs (later overrides earlier)
	merged := configs[0]
	for i := 1; i < len(configs); i++ {
		merged = mergeConfigs(merged, configs[i])
	}

	return merged, nil
}

func searchPaths() []string {
	var paths []string

	paths = append(paths, ".ccchain.conf")
	paths = append(paths, ".ccchain.local.conf")

	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		paths = append(paths, filepath.Join(dir, "ccchain.conf"))
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			paths = append(paths, filepath.Join(home, ".claude", "ccchain.conf"))
		}
	}

	return paths
}

func loadAndParse(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	config, err := Parse(f)
	if err != nil {
		return nil, err
	}

	if err := ResolveTemplates(config); err != nil {
		return nil, err
	}

	return config, nil
}

func mergeConfigs(base, overlay *Config) *Config {
	merged := &Config{
		Templates: mergeSlice(base.Templates, overlay.Templates),
		PreRules:  mergeSlice(base.PreRules, overlay.PreRules),
		PostRules: mergeSlice(base.PostRules, overlay.PostRules),
		Rules:     mergeSlice(base.Rules, overlay.Rules),
		Settings:  overlay.Settings,
	}

	if overlay.Settings == nil {
		merged.Settings = base.Settings
	}

	return merged
}

func mergeSlice[T any](a, b []T) []T {
	out := make([]T, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}
