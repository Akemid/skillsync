package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Tool represents a supported agentic coding tool
type Tool struct {
	Name       string `yaml:"name"`
	GlobalPath string `yaml:"global_path"` // e.g. ~/.claude/skills
	LocalPath  string `yaml:"local_path"`  // e.g. .claude/skills
	Enabled    bool   `yaml:"enabled"`
}

// SkillRef references a skill in the registry with optional overrides
type SkillRef struct {
	Name string   `yaml:"name"`
	Tags []string `yaml:"tags,omitempty"`
}

// Bundle is a named group of skills (e.g. "CEN", "personal")
type Bundle struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description,omitempty"`
	Company     string     `yaml:"company,omitempty"`
	Tags        []string   `yaml:"tags,omitempty"`
	Tech        []string   `yaml:"tech,omitempty"` // technologies this bundle applies to
	Skills      []SkillRef `yaml:"skills"`
}

// Config is the top-level skillsync configuration
type Config struct {
	RegistryPath string   `yaml:"registry_path"` // path to skill registry (e.g. ~/.agents/skills)
	Bundles      []Bundle `yaml:"bundles"`
	Tools        []Tool   `yaml:"tools"`
}

// DefaultTools returns the pre-configured list of agentic tools
func DefaultTools() []Tool {
	return []Tool{
		{Name: "claude", GlobalPath: "~/.claude/skills", LocalPath: ".claude/skills", Enabled: true},
		{Name: "copilot", GlobalPath: "~/.copilot/skills", LocalPath: ".copilot/skills", Enabled: true},
		{Name: "codex", GlobalPath: "~/.codex/skills", LocalPath: ".codex/skills", Enabled: true},
		{Name: "kiro", GlobalPath: "~/.kiro/skills", LocalPath: ".kiro/skills", Enabled: true},
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
	return &cfg, nil
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
