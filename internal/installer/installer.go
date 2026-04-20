package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Akemid/skillsync/internal/config"
	"github.com/Akemid/skillsync/internal/registry"
)

// Scope determines where skills are installed
type Scope int

const (
	ScopeGlobal  Scope = iota // Install to global tool directories (~/.claude/skills, etc.)
	ScopeProject              // Install to project-local directories (.claude/skills, etc.)
)

// Result captures the outcome of a single install operation
type Result struct {
	Tool    string
	Skill   string
	Target  string
	Created bool
	Existed bool
	Error   error
}

// Install creates symlinks for the given skills into the selected tools' directories
func Install(skills []registry.Skill, tools []config.Tool, scope Scope, projectDir string) []Result {
	var results []Result

	for _, tool := range tools {
		var basePath string
		switch scope {
		case ScopeGlobal:
			basePath = config.ExpandPath(tool.GlobalPath)
		case ScopeProject:
			basePath = filepath.Join(projectDir, tool.LocalPath)
		}

		for _, skill := range skills {
			target := filepath.Join(basePath, skill.Name)
			res := Result{
				Tool:   tool.Name,
				Skill:  skill.Name,
				Target: target,
			}

			// Check if already exists
			if info, err := os.Lstat(target); err == nil {
				// If it's already a symlink pointing to the same source, skip
				if info.Mode()&os.ModeSymlink != 0 {
					linkTarget, _ := os.Readlink(target)
					absLink, _ := filepath.Abs(filepath.Join(filepath.Dir(target), linkTarget))
					absSkill, _ := filepath.Abs(skill.Path)
					if absLink == absSkill {
						res.Existed = true
						results = append(results, res)
						continue
					}
				}
				res.Existed = true
				results = append(results, res)
				continue
			}

			// Ensure parent directory exists
			if err := os.MkdirAll(basePath, 0755); err != nil {
				res.Error = fmt.Errorf("creating dir %s: %w", basePath, err)
				results = append(results, res)
				continue
			}

			// Create relative symlink
			relPath, err := filepath.Rel(basePath, skill.Path)
			if err != nil {
				// Fall back to absolute path
				relPath = skill.Path
			}

			if err := os.Symlink(relPath, target); err != nil {
				res.Error = fmt.Errorf("symlink: %w", err)
			} else {
				res.Created = true
			}
			results = append(results, res)
		}
	}

	return results
}

// Uninstall removes symlinks for the given skills from the selected tools
func Uninstall(skillNames []string, tools []config.Tool, scope Scope, projectDir string) []Result {
	var results []Result

	for _, tool := range tools {
		var basePath string
		switch scope {
		case ScopeGlobal:
			basePath = config.ExpandPath(tool.GlobalPath)
		case ScopeProject:
			basePath = filepath.Join(projectDir, tool.LocalPath)
		}

		for _, name := range skillNames {
			target := filepath.Join(basePath, name)
			res := Result{
				Tool:   tool.Name,
				Skill:  name,
				Target: target,
			}

			info, err := os.Lstat(target)
			if err != nil {
				res.Error = fmt.Errorf("not found")
				results = append(results, res)
				continue
			}

			// Only remove symlinks, never real directories
			if info.Mode()&os.ModeSymlink == 0 {
				res.Error = fmt.Errorf("not a symlink, skipping for safety")
				results = append(results, res)
				continue
			}

			if err := os.Remove(target); err != nil {
				res.Error = err
			} else {
				res.Created = true // reuse field to indicate successful removal
			}
			results = append(results, res)
		}
	}

	return results
}
