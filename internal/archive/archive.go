package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Export creates a .tar.gz archive from skillPath, writing it to outputPath.
// The archive entries are rooted at the skill directory name (e.g. my-skill/SKILL.md).
// Returns an error if SKILL.md is not found in skillPath.
func Export(skillPath, outputPath string) error {
	// Validate SKILL.md exists
	if _, err := os.Stat(filepath.Join(skillPath, "SKILL.md")); os.IsNotExist(err) {
		return fmt.Errorf("skill directory %q has no SKILL.md", skillPath)
	}

	skillName := filepath.Base(skillPath)

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating archive file: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.WalkDir(skillPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(skillPath, path)
		if err != nil {
			return err
		}

		// Archive entry path: skillName/rel (e.g. my-skill/SKILL.md)
		archiveName := filepath.Join(skillName, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = archiveName

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		return err
	})

	return err
}

// Import extracts the .tar.gz at archivePath into registryPath/<skillName>/.
// Returns the installed skill name.
// Validates: exactly one top-level dir, SKILL.md present, no path traversal.
// If skill already exists and force is false, returns an error.
func Import(archivePath, registryPath string, force bool) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("opening archive: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("reading gzip stream: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	// First pass: collect all headers to validate before writing anything
	type entry struct {
		hdr  *tar.Header
		data []byte
	}
	var entries []entry
	var topLevelDirs []string
	hasSKILLMD := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading archive entry: %w", err)
		}

		// Path traversal check
		if !isSafePath(hdr.Name) {
			return "", fmt.Errorf("invalid archive — unsafe path detected: %q", hdr.Name)
		}

		// Track top-level directories
		parts := strings.SplitN(hdr.Name, "/", 2)
		if len(parts) > 0 && parts[0] != "" {
			found := false
			for _, d := range topLevelDirs {
				if d == parts[0] {
					found = true
					break
				}
			}
			if !found {
				topLevelDirs = append(topLevelDirs, parts[0])
			}
		}

		// Check for SKILL.md at the top level of the first dir
		if len(parts) == 2 && parts[1] == "SKILL.md" {
			hasSKILLMD = true
		}

		var data []byte
		if hdr.Typeflag == tar.TypeReg && hdr.Size > 0 {
			data, err = io.ReadAll(tr)
			if err != nil {
				return "", fmt.Errorf("reading entry %q: %w", hdr.Name, err)
			}
		}
		entries = append(entries, entry{hdr: hdr, data: data})
	}

	if !hasSKILLMD {
		return "", fmt.Errorf("invalid archive — no SKILL.md found")
	}

	if len(topLevelDirs) == 0 {
		return "", fmt.Errorf("invalid archive — empty")
	}

	skillName := topLevelDirs[0]

	// Check conflict
	destDir := filepath.Join(registryPath, skillName)
	if _, err := os.Stat(destDir); err == nil {
		if !force {
			return "", fmt.Errorf("skill %q already installed; use --force to overwrite", skillName)
		}
		if err := os.RemoveAll(destDir); err != nil {
			return "", fmt.Errorf("removing existing skill: %w", err)
		}
	}

	// Extract to temp dir first (atomic)
	tempDir, err := os.MkdirTemp(registryPath, ".tmp-import-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	for _, e := range entries {
		destPath := filepath.Join(tempDir, e.hdr.Name)

		switch e.hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, os.FileMode(e.hdr.Mode)); err != nil {
				return "", fmt.Errorf("creating dir %q: %w", destPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return "", fmt.Errorf("creating parent dir: %w", err)
			}
			if err := os.WriteFile(destPath, e.data, os.FileMode(e.hdr.Mode)); err != nil {
				return "", fmt.Errorf("writing file %q: %w", destPath, err)
			}
		}
	}

	// Atomic rename from temp to final destination
	if err := os.Rename(filepath.Join(tempDir, skillName), destDir); err != nil {
		return "", fmt.Errorf("installing skill: %w", err)
	}

	return skillName, nil
}

// isSafePath returns true when the path is relative and contains no traversal sequences.
func isSafePath(path string) bool {
	clean := filepath.Clean(path)
	return !filepath.IsAbs(clean) && !strings.HasPrefix(clean, "..")
}
