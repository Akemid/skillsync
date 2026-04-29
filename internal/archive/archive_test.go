package archive

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// makeSkillDir creates base/name/SKILL.md and base/name/scripts/run.sh
func makeSkillDir(t *testing.T, base, name string) string {
	t.Helper()
	skillDir := filepath.Join(base, name)
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: "+name+"\ndescription: Test skill\n---\n# "+name), 0644); err != nil {
		t.Fatalf("WriteFile SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "run.sh"),
		[]byte("#!/bin/sh\necho hello"), 0644); err != nil {
		t.Fatalf("WriteFile run.sh: %v", err)
	}
	return skillDir
}

// TestExport_Success verifies that Export produces a valid .tar.gz with the full
// file tree rooted at the skill name directory.
func TestExport_Success(t *testing.T) {
	base := t.TempDir()
	skillDir := makeSkillDir(t, base, "my-skill")

	outputPath := filepath.Join(t.TempDir(), "my-skill.tar.gz")

	if err := Export(skillDir, outputPath); err != nil {
		t.Fatalf("Export() error = %v, want nil", err)
	}

	// Verify file exists and is non-empty
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat output file: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	// Read and verify archive contents
	f, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	found := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		found[hdr.Name] = true
	}

	for _, expected := range []string{"my-skill/SKILL.md", "my-skill/scripts/run.sh"} {
		if !found[expected] {
			t.Errorf("archive missing entry %q; got %v", expected, found)
		}
	}
}

// TestExport_MissingSkillMD verifies that a directory without SKILL.md returns an error.
func TestExport_MissingSkillMD(t *testing.T) {
	base := t.TempDir()
	skillDir := filepath.Join(base, "bad-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// No SKILL.md

	outputPath := filepath.Join(t.TempDir(), "bad-skill.tar.gz")

	err := Export(skillDir, outputPath)
	if err == nil {
		t.Fatal("Export() error = nil, want error for missing SKILL.md")
	}
}

// TestImport_Success exports a skill then imports it to a fresh registry directory
// and verifies that registry/<skillname>/SKILL.md exists.
func TestImport_Success(t *testing.T) {
	base := t.TempDir()
	skillDir := makeSkillDir(t, base, "test-skill")
	archivePath := filepath.Join(t.TempDir(), "test-skill.tar.gz")

	if err := Export(skillDir, archivePath); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	registry := t.TempDir()
	skillName, err := Import(archivePath, registry, false)
	if err != nil {
		t.Fatalf("Import() error = %v, want nil", err)
	}

	if skillName != "test-skill" {
		t.Errorf("skillName = %q, want %q", skillName, "test-skill")
	}

	installedSKILL := filepath.Join(registry, "test-skill", "SKILL.md")
	if _, err := os.Stat(installedSKILL); os.IsNotExist(err) {
		t.Errorf("SKILL.md not found at %s after import", installedSKILL)
	}
}

// TestImport_PathTraversal verifies that archives containing unsafe paths are
// rejected before any files are written.
func TestImport_PathTraversal(t *testing.T) {
	// Craft a malicious tar.gz with a path traversal entry
	archivePath := filepath.Join(t.TempDir(), "malicious.tar.gz")
	writeMaliciousArchive(t, archivePath)

	registry := t.TempDir()
	_, err := Import(archivePath, registry, false)
	if err == nil {
		t.Fatal("Import() error = nil, want error for path traversal")
	}
	if !containsStr(err.Error(), "unsafe path") {
		t.Errorf("error = %q, want to contain 'unsafe path'", err.Error())
	}

	// Verify nothing was written to registry
	entries, _ := os.ReadDir(registry)
	if len(entries) != 0 {
		t.Errorf("registry has %d entries after path traversal rejection, want 0", len(entries))
	}
}

// writeMaliciousArchive creates a tar.gz with a path traversal entry.
func writeMaliciousArchive(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create malicious archive: %v", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	content := []byte("evil")
	hdr := &tar.Header{
		Name: "../../etc/passwd",
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tw.Write: %v", err)
	}
}

// TestImport_MissingSkillMD verifies that an archive without SKILL.md returns an
// appropriate error and writes nothing to the registry.
func TestImport_MissingSkillMD(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "no-skillmd.tar.gz")
	writeArchiveWithoutSKILLMD(t, archivePath)

	registry := t.TempDir()
	_, err := Import(archivePath, registry, false)
	if err == nil {
		t.Fatal("Import() error = nil, want error when SKILL.md is missing")
	}
	if !containsStr(err.Error(), "no SKILL.md") {
		t.Errorf("error = %q, want to contain 'no SKILL.md'", err.Error())
	}

	entries, _ := os.ReadDir(registry)
	if len(entries) != 0 {
		t.Errorf("registry has %d entries, want 0", len(entries))
	}
}

func writeArchiveWithoutSKILLMD(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	content := []byte("some script")
	hdr := &tar.Header{
		Name: "my-skill/scripts/run.sh",
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tw.Write: %v", err)
	}
}

// TestImport_Conflict_NoForce verifies that importing a skill that already exists
// in the registry without force returns an "already installed" error.
func TestImport_Conflict_NoForce(t *testing.T) {
	base := t.TempDir()
	skillDir := makeSkillDir(t, base, "existing-skill")
	archivePath := filepath.Join(t.TempDir(), "existing-skill.tar.gz")

	if err := Export(skillDir, archivePath); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	registry := t.TempDir()

	// Pre-install the skill
	installed := filepath.Join(registry, "existing-skill")
	if err := os.MkdirAll(installed, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installed, "SKILL.md"), []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Import(archivePath, registry, false)
	if err == nil {
		t.Fatal("Import() error = nil, want error when skill already installed")
	}
	if !containsStr(err.Error(), "already installed") {
		t.Errorf("error = %q, want to contain 'already installed'", err.Error())
	}
}

// TestImport_Conflict_Force verifies that importing with force=true succeeds even
// when the skill already exists.
func TestImport_Conflict_Force(t *testing.T) {
	base := t.TempDir()
	skillDir := makeSkillDir(t, base, "force-skill")
	archivePath := filepath.Join(t.TempDir(), "force-skill.tar.gz")

	if err := Export(skillDir, archivePath); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	registry := t.TempDir()

	// Pre-install the skill
	installed := filepath.Join(registry, "force-skill")
	if err := os.MkdirAll(installed, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installed, "SKILL.md"), []byte("old content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	skillName, err := Import(archivePath, registry, true)
	if err != nil {
		t.Fatalf("Import() with force=true error = %v, want nil", err)
	}
	if skillName != "force-skill" {
		t.Errorf("skillName = %q, want %q", skillName, "force-skill")
	}

	// Verify content was overwritten (new SKILL.md has different content)
	data, err := os.ReadFile(filepath.Join(registry, "force-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) == "old content" {
		t.Error("SKILL.md was not overwritten after force import")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
