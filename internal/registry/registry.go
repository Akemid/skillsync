package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Akemid/skillsync/internal/config"
	"gopkg.in/yaml.v3"
)

// SkillMeta holds the YAML frontmatter from a SKILL.md
type SkillMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Skill represents a discovered skill in the registry
type Skill struct {
	Name        string
	Description string
	Path        string   // absolute path to the skill folder
	Files       []string // list of files in the skill folder
}

// Registry manages skills from a central directory
type Registry struct {
	BasePath string
	Skills   []Skill
}

// New creates a Registry from the given base path (e.g. ~/.agents/skills)
func New(basePath string) *Registry {
	return &Registry{
		BasePath: config.ExpandPath(basePath),
	}
}

// Discover scans the registry directory and loads all skills
// Scans both local skills (basePath) and remote bundles (_remote/)
func (r *Registry) Discover() error {
	entries, err := os.ReadDir(r.BasePath)
	if err != nil {
		return fmt.Errorf("reading registry %s: %w", r.BasePath, err)
	}

	r.Skills = nil

	// Scan local skills (skip _remote directory)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "_remote" {
			continue
		}
		skillPath := filepath.Join(r.BasePath, entry.Name())
		skill := r.scanSkillDir(skillPath, entry.Name())
		r.Skills = append(r.Skills, skill)
	}

	// Scan remote bundles in _remote/*/
	remoteDir := filepath.Join(r.BasePath, "_remote")
	if remoteEntries, err := os.ReadDir(remoteDir); err == nil {
		for _, bundleEntry := range remoteEntries {
			if !bundleEntry.IsDir() || strings.HasPrefix(bundleEntry.Name(), ".") {
				continue
			}
			bundlePath := filepath.Join(remoteDir, bundleEntry.Name())

			// Scan skills inside bundle directory
			if skillEntries, err := os.ReadDir(bundlePath); err == nil {
				for _, skillEntry := range skillEntries {
					if !skillEntry.IsDir() || strings.HasPrefix(skillEntry.Name(), ".") {
						continue
					}
					skillPath := filepath.Join(bundlePath, skillEntry.Name())
					skill := r.scanSkillDir(skillPath, skillEntry.Name())
					r.Skills = append(r.Skills, skill)
				}
			}
		}
	}
	// Note: Missing _remote/ is not an error

	return nil
}

// scanSkillDir scans a single skill directory and returns a Skill
func (r *Registry) scanSkillDir(skillPath, skillName string) Skill {
	skillMD := filepath.Join(skillPath, "SKILL.md")

	skill := Skill{
		Name: skillName,
		Path: skillPath,
	}

	// Try to read SKILL.md for metadata
	if data, err := os.ReadFile(skillMD); err == nil {
		if meta, err := parseFrontmatter(data); err == nil {
			if meta.Description != "" {
				skill.Description = meta.Description
			}
		}
	}

	// List files in the skill folder
	if files, err := listFiles(skillPath); err == nil {
		skill.Files = files
	}

	return skill
}

// FindByNames returns skills matching the given names
func (r *Registry) FindByNames(names []string) []Skill {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	var result []Skill
	for _, s := range r.Skills {
		if nameSet[s.Name] {
			result = append(result, s)
		}
	}
	return result
}

// parseFrontmatter extracts YAML frontmatter from a markdown file
func parseFrontmatter(data []byte) (*SkillMeta, error) {
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("no frontmatter found")
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return nil, fmt.Errorf("no frontmatter closing")
	}
	var meta SkillMeta
	if err := yaml.Unmarshal([]byte(content[4:4+end]), &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// listFiles returns relative file paths inside a directory
func listFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}
