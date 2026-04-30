package skillasset

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed skill/skillsync/SKILL.md
var skillMD []byte

// SkillName is the canonical name of the embedded skill.
const SkillName = "skillsync"

// Content returns the raw embedded SKILL.md bytes.
func Content() []byte { return skillMD }

// ExtractTo writes the embedded SKILL.md to destDir/skillsync/SKILL.md.
// destDir must already exist — ExtractTo does NOT create it.
// Creates the skillsync/ subdirectory. Always overwrites (O_TRUNC semantics).
// Returns a non-nil error if destDir does not exist or any I/O operation fails.
func ExtractTo(destDir string) error {
	skillDir := filepath.Join(destDir, SkillName)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("creating skill dir: %w", err)
	}

	destPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(destPath, skillMD, 0644); err != nil {
		return fmt.Errorf("writing SKILL.md: %w", err)
	}

	return nil
}
