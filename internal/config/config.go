package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Tool represents a supported agentic coding tool
type Tool struct {
	Name        string `yaml:"name"`
	GlobalPath  string `yaml:"global_path"` // e.g. ~/.claude/skills
	LocalPath   string `yaml:"local_path"`  // e.g. .claude/skills
	Enabled     bool   `yaml:"enabled"`
	InstallMode string `yaml:"install_mode,omitempty"` // "copy" or "symlink" (default)
}

// IsCopyMode returns true when the tool uses file copy instead of symlinks
func (t Tool) IsCopyMode() bool {
	return t.InstallMode == "copy"
}

// SkillRef references a skill in the registry with optional overrides
type SkillRef struct {
	Name string   `yaml:"name"`
	Tags []string `yaml:"tags,omitempty"`
}

// Source represents a remote Git source for a bundle
type Source struct {
	Type   string `yaml:"type"`   // MUST be "git"
	URL    string `yaml:"url"`    // Git clone URL (HTTPS or SSH)
	Branch string `yaml:"branch"` // Optional, defaults to "main"
	Path   string `yaml:"path"`   // Optional subdirectory within repo
}

// Bundle is a named group of skills (e.g. "CEN", "personal")
type Bundle struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description,omitempty"`
	Company     string     `yaml:"company,omitempty"`
	Tags        []string   `yaml:"tags,omitempty"`
	Tech        []string   `yaml:"tech,omitempty"`    // technologies this bundle applies to
	Source      *Source    `yaml:"source,omitempty"` // Optional Git source, nil = local-only
	SSHKey      string     `yaml:"ssh_key,omitempty"` // Optional SSH key path for private repos
	Skills      []SkillRef `yaml:"skills"`
}

// Tap represents a writable git repository for uploading skills
type Tap struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Branch string `yaml:"branch"`
	SSHKey string `yaml:"ssh_key,omitempty"` // Optional SSH key path for private repos
}

// Config is the top-level skillsync configuration
type Config struct {
	RegistryPath string   `yaml:"registry_path"` // path to skill registry (e.g. ~/.agents/skills)
	Bundles      []Bundle `yaml:"bundles"`
	Tools        []Tool   `yaml:"tools"`
	Taps         []Tap    `yaml:"taps,omitempty"`
}

// MigrationSummary reports what changed during upgrade-config.
type MigrationSummary struct {
	Changed        bool
	MigratedLegacy bool
	AddedTools     []string
	PreservedTools []string
	Unchanged      bool
}

// DefaultBundles returns the pre-configured list of well-known remote bundles.
// These are included when running `skillsync init` so users don't need to know the URLs.
func DefaultBundles() []Bundle {
	return []Bundle{
		{
			Name:        "frontend-cen",
			Description: "CEN frontend AI skills",
			Company:     "CEN",
			Tags:        []string{"cen", "frontend"},
			Source: &Source{
				Type:   "git",
				URL:    "https://github.com/Akemid/cen-ai-tools",
				Branch: "main",
				Path:   "skills",
			},
		},
	}
}

// DefaultTools returns the pre-configured list of agentic tools
func DefaultTools() []Tool {
	return []Tool{
		{Name: "claude", GlobalPath: "~/.claude/skills", LocalPath: ".claude/skills", Enabled: true},
		{Name: "copilot", GlobalPath: "~/.copilot/skills", LocalPath: ".github/skills", Enabled: true},
		{Name: "codex", GlobalPath: "~/.codex/skills", LocalPath: ".codex/skills", Enabled: true},
		{Name: "kiro-ide", GlobalPath: "~/.kiro/skills", LocalPath: ".kiro/skills", Enabled: true, InstallMode: "copy"},
		{Name: "kiro-cli", GlobalPath: "~/.kiro/skills", LocalPath: ".kiro/skills", Enabled: false, InstallMode: "symlink"},
		{Name: "gemini", GlobalPath: "~/.gemini/skills", LocalPath: ".gemini/skills", Enabled: true},
		{Name: "cursor", GlobalPath: "~/.cursor/skills", LocalPath: ".cursor/skills", Enabled: false},
		{Name: "roo-code", GlobalPath: "~/.roo-code/skills", LocalPath: ".roo-code/skills", Enabled: false},
		{Name: "junie", GlobalPath: "~/.junie/skills", LocalPath: ".junie/skills", Enabled: false},
		{Name: "trae", GlobalPath: "~/.trae/skills", LocalPath: ".trae/skills", Enabled: false},
	}
}

// ExpandPath expands ~ to home directory
func ExpandPath(p string) string {
	if len(p) > 1 && p[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

// Load reads the config from the given path
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	// Set defaults if not specified
	if cfg.RegistryPath == "" {
		cfg.RegistryPath = "~/.agents/skills"
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = DefaultTools()
	}
	// Validate bundle sources
	for _, bundle := range cfg.Bundles {
		if bundle.Source != nil {
			if err := validateSource(bundle.Source); err != nil {
				return nil, fmt.Errorf("bundle %q: %w", bundle.Name, err)
			}
		}
	}
	return &cfg, nil
}

// validateSource checks if a Source configuration is valid
func validateSource(src *Source) error {
	if src.Type != "git" {
		return fmt.Errorf("unsupported source type %q (must be \"git\")", src.Type)
	}
	if src.URL == "" {
		return fmt.Errorf("source.url is required")
	}
	// Validate URL format (basic check for Git URLs)
	if !strings.HasPrefix(src.URL, "https://") &&
		!strings.HasPrefix(src.URL, "git://") &&
		!strings.HasPrefix(src.URL, "git@") {
		return fmt.Errorf("invalid Git URL %q (must start with https://, git://, or git@)", src.URL)
	}
	return nil
}

// DefaultConfigPath returns the default config file location
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "skillsync.yaml"
	}
	return filepath.Join(home, ".config", "skillsync", "skillsync.yaml")
}

// Save writes the config to the given path
func Save(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// MigrateTools applies non-destructive migration rules to existing tool entries.
// It preserves unknown tools, migrates legacy "kiro" entries, and avoids duplicates.
func MigrateTools(existing []Tool) ([]Tool, MigrationSummary) {
	migrated := make([]Tool, 0, len(existing)+2)
	summary := MigrationSummary{}

	var legacyKiro *Tool
	hasKiroIDE := false
	hasKiroCLI := false

	for i := range existing {
		tool := existing[i]
		switch tool.Name {
		case "kiro":
			if legacyKiro == nil {
				copy := tool
				legacyKiro = &copy
			}
			summary.MigratedLegacy = true
			summary.Changed = true
			continue
		case "kiro-ide":
			hasKiroIDE = true
		case "kiro-cli":
			hasKiroCLI = true
		}
		migrated = append(migrated, tool)
		summary.PreservedTools = append(summary.PreservedTools, tool.Name)
	}

	if summary.MigratedLegacy {
		if !hasKiroIDE {
			migrated = append(migrated, Tool{
				Name:        "kiro-ide",
				GlobalPath:  legacyKiro.GlobalPath,
				LocalPath:   legacyKiro.LocalPath,
				Enabled:     true,
				InstallMode: "copy",
			})
			summary.AddedTools = append(summary.AddedTools, "kiro-ide")
			summary.Changed = true
		}

		if !hasKiroCLI {
			migrated = append(migrated, Tool{
				Name:        "kiro-cli",
				GlobalPath:  legacyKiro.GlobalPath,
				LocalPath:   legacyKiro.LocalPath,
				Enabled:     false,
				InstallMode: "symlink",
			})
			summary.AddedTools = append(summary.AddedTools, "kiro-cli")
			summary.Changed = true
		}
	}

	summary.Unchanged = !summary.Changed
	return migrated, summary
}
