package templates

import "embed"

// FS is the embedded filesystem containing all dieuctl code-generation templates.
// Templates are compiled into the binary at build time — no external files needed at runtime.
//
//go:embed cluster
var FS embed.FS
