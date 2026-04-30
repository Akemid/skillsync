package installer

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
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

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copyFile %s→%s: %w", src, dst, err)
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("copyFile %s→%s: %w", src, dst, err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("copyFile %s→%s: %w", src, dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copyFile %s→%s: %w", src, dst, err)
	}

	return nil
}

func copySkillDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("copySkillDir %s→%s: %w", src, dst, err)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("copySkillDir %s→%s: %w", src, dst, err)
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("copySkillDir %s→%s: %w", src, dst, err)
		}
		if relPath == "." {
			return nil
		}

		targetPath := filepath.Join(dst, relPath)
		if d.IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("copySkillDir %s→%s: %w", src, dst, err)
			}
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		return copyFile(path, targetPath)
	})
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
				if tool.IsCopyMode() {
					if info.Mode()&os.ModeSymlink != 0 {
						res.Error = fmt.Errorf("conflict: symlink found where directory expected for copy-mode tool %s", tool.Name)
						results = append(results, res)
						continue
					}

					if !info.IsDir() {
						res.Error = fmt.Errorf("conflict: non-directory found where copy-mode directory expected for tool %s", tool.Name)
						results = append(results, res)
						continue
					}

					if _, err := os.Stat(filepath.Join(target, "SKILL.md")); err == nil {
						res.Existed = true
					} else if errors.Is(err, os.ErrNotExist) {
						res.Error = fmt.Errorf("conflict: directory exists without SKILL.md sentinel for copy-mode tool %s", tool.Name)
					} else {
						res.Error = fmt.Errorf("checking SKILL.md sentinel in %s: %w", target, err)
					}
					results = append(results, res)
					continue
				}

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
					res.Existed = true
					results = append(results, res)
					continue
				}

				if info.IsDir() {
					res.Error = fmt.Errorf("conflict: directory found where symlink expected for tool %s", tool.Name)
					results = append(results, res)
					continue
				}

				res.Existed = true
				results = append(results, res)
				continue
			} else if !errors.Is(err, os.ErrNotExist) {
				res.Error = fmt.Errorf("checking target %s: %w", target, err)
				results = append(results, res)
				continue
			}

			// Ensure parent directory exists
			if err := os.MkdirAll(basePath, 0755); err != nil {
				res.Error = fmt.Errorf("creating dir %s: %w", basePath, err)
				results = append(results, res)
				continue
			}

			if tool.IsCopyMode() {
				if err := copySkillDir(skill.Path, target); err != nil {
					res.Error = err
				} else {
					res.Created = true
				}
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

			if tool.IsCopyMode() {
				if info.Mode()&os.ModeSymlink == 0 {
					if !info.IsDir() {
						res.Error = fmt.Errorf("refusing to remove %s: not a directory or symlink", target)
						results = append(results, res)
						continue
					}

					if _, err := os.Stat(filepath.Join(target, "SKILL.md")); err == nil {
						if err := os.RemoveAll(target); err != nil {
							res.Error = err
						} else {
							res.Created = true
						}
					} else if errors.Is(err, os.ErrNotExist) {
						res.Error = fmt.Errorf("refusing to remove %s: no SKILL.md sentinel", target)
					} else {
						res.Error = fmt.Errorf("checking SKILL.md sentinel in %s: %w", target, err)
					}
					results = append(results, res)
					continue
				}
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
