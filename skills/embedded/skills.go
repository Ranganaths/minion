// Package embedded provides built-in default skills for the Minion framework.
// These skills are embedded directly in the binary and loaded automatically
// unless explicitly disabled.
package embedded

import (
	"embed"
)

// SkillsFS contains the embedded skill files
//
//go:embed *.md
var SkillsFS embed.FS

// SkillsPattern is the glob pattern for skill files
const SkillsPattern = "*.md"
