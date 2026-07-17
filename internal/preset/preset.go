// Package preset ships curated ccchain rulesets that are embedded into the
// binary at build time. The presets are used by `ccchain init [--sentinel]`
// and are also loaded by fixture tests so that the shipped config and the
// tested config are guaranteed to be the same bytes (no manual sync drift).
package preset

import _ "embed"

//go:embed sentinel.conf
var sentinelConfig string

// Sentinel returns the deny-first sentinel preset shipped with
// `ccchain init --sentinel`. See Plan 0022 Phase 4 for the rationale and
// the collection strategy.
func Sentinel() string { return sentinelConfig }
