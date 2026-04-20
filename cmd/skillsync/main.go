package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sergiomondragonsilva/skillsync/internal/config"
	"github.com/sergiomondragonsilva/skillsync/internal/installer"
	"github.com/sergiomondragonsilva/skillsync/internal/registry"
	"github.com/sergiomondragonsilva/skillsync/internal/sync"
	"github.com/sergiomondragonsilva/skillsync/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Handle help early (before config loading)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h", "help":
			printUsage()
			return nil
		}
	}

	// Determine config path
	configPath := config.DefaultConfigPath()
	if envPath := os.Getenv("SKILLSYNC_CONFIG"); envPath != "" {
		configPath = envPath
	}
	// Allow --config flag
	for i, arg := range os.Args[1:] {
		if arg == "--config" && i+1 < len(os.Args[1:]) {
			configPath = os.Args[i+2]
		}
	}

	// Handle init early (only needs defaults)
	if len(os.Args) > 1 && os.Args[1] == "init" {
		cfg := &config.Config{
			RegistryPath: "~/.agents/skills",
			Tools:        config.DefaultTools(),
		}
		return cmdInit(cfg, configPath)
	}

	// Load or create config
	cfg, err := config.Load(configPath)
	if err != nil {
		// If config doesn't exist, use defaults
		if errors.Is(err, os.ErrNotExist) {
			cfg = &config.Config{
				RegistryPath: "~/.agents/skills",
				Tools:        config.DefaultTools(),
			}
		} else {
			return fmt.Errorf("loading config from %s: %w", configPath, err)
		}
	}

	// Auto-detect which tools are installed
	cfg.Tools = tui.DetectInstalledTools(cfg.Tools)

	// Initialize registry
	reg := registry.New(cfg.RegistryPath)
	if err := reg.Discover(); err != nil {
		return fmt.Errorf("scanning skill registry: %w", err)
	}

	if len(reg.Skills) == 0 {
		return fmt.Errorf("no skills found in registry at %s\nAdd skills to your registry or update registry_path in %s",
			config.ExpandPath(cfg.RegistryPath), configPath)
	}

	// Get current working directory for project-scoped installs
	projectDir, _ := os.Getwd()

	// Handle subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "list":
			return cmdList(reg)
		case "status":
			return cmdStatus(cfg, reg)
		case "sync":
			return cmdSync(cfg)
		case "uninstall":
			return cmdUninstall(cfg, reg, projectDir)
		case "--help", "-h", "help":
			printUsage()
			return nil
		case "--config":
			// handled above, fall through to wizard
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
			printUsage()
			return nil
		}
	}

	// Run interactive wizard
	result, err := tui.RunWizard(cfg, reg, projectDir)
	if err != nil {
		return err
	}

	// Resolve selected tools
	var selectedTools []config.Tool
	for _, name := range result.SelectedTools {
		for _, t := range cfg.Tools {
			if t.Name == name {
				selectedTools = append(selectedTools, t)
				break
			}
		}
	}

	// Resolve selected skills
	skills := reg.FindByNames(result.SelectedSkills)

	// Install
	results := installer.Install(skills, selectedTools, result.Scope, result.ProjectDir)
	tui.PrintResults(results)

	return nil
}

func cmdList(reg *registry.Registry) error {
	fmt.Printf("Skills in registry (%s):\n\n", reg.BasePath)
	for _, s := range reg.Skills {
		if s.Description != "" {
			fmt.Printf("  %-25s %s\n", s.Name, s.Description)
		} else {
			fmt.Printf("  %s\n", s.Name)
		}
	}
	fmt.Printf("\n  Total: %d skills\n", len(reg.Skills))
	return nil
}

func cmdStatus(cfg *config.Config, reg *registry.Registry) error {
	fmt.Println("Installed skills per tool:")
	for _, tool := range cfg.Tools {
		globalPath := config.ExpandPath(tool.GlobalPath)
		entries, err := os.ReadDir(globalPath)
		if err != nil {
			continue
		}
		fmt.Printf("  %s (%s):\n", tool.Name, globalPath)
		for _, e := range entries {
			if !e.IsDir() && e.Type()&os.ModeSymlink == 0 {
				continue
			}
			info, _ := e.Info()
			marker := "  "
			if info != nil && info.Mode()&os.ModeSymlink != 0 {
				target, _ := os.Readlink(fmt.Sprintf("%s/%s", globalPath, e.Name()))
				marker = fmt.Sprintf("→ %s", target)
			}
			fmt.Printf("    %s %s\n", e.Name(), marker)
		}
	}
	_ = reg // available for future cross-referencing
	return nil
}

func cmdUninstall(cfg *config.Config, reg *registry.Registry, projectDir string) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: skillsync uninstall <skill-name> [--global|--project]")
	}
	skillName := os.Args[2]
	scope := installer.ScopeGlobal
	for _, arg := range os.Args[3:] {
		if arg == "--project" {
			scope = installer.ScopeProject
		}
	}
	results := installer.Uninstall([]string{skillName}, cfg.Tools, scope, projectDir)
	tui.PrintResults(results)
	_ = reg
	return nil
}

func cmdSync(cfg *config.Config) error {
	// Count bundles with a remote source
	var remoteBundles []config.Bundle
	for _, b := range cfg.Bundles {
		if b.Source != nil {
			remoteBundles = append(remoteBundles, b)
		}
	}

	if len(remoteBundles) == 0 {
		fmt.Println("No remote bundles configured.")
		return nil
	}

	fmt.Printf("Syncing %d remote bundle(s)...\n", len(remoteBundles))

	// Initialize syncer with _remote/ under the registry
	remoteBaseDir := filepath.Join(config.ExpandPath(cfg.RegistryPath), "_remote")
	syncer, err := sync.New(remoteBaseDir)
	if err != nil {
		return fmt.Errorf("initializing syncer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var anyFailed bool
	for _, bundle := range remoteBundles {
		if bundle.Source.Type != "git" {
			fmt.Fprintf(os.Stderr, "  ✗ %s skipped: unsupported source type %q (only \"git\" is supported)\n",
				bundle.Name, bundle.Source.Type)
			anyFailed = true
			continue
		}

		fmt.Printf("  Syncing %s from %s...\n", bundle.Name, bundle.Source.URL)
		if err := syncer.SyncBundle(ctx, bundle.Name, bundle.Source.URL, bundle.Source.Branch); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s failed: %v\n", bundle.Name, err)
			anyFailed = true
		} else {
			fmt.Printf("  ✓ %s synced\n", bundle.Name)
		}
	}

	if anyFailed {
		os.Exit(1)
	}
	return nil
}

func cmdInit(cfg *config.Config, configPath string) error {
	if err := config.Save(cfg, configPath); err != nil {
		return err
	}
	fmt.Printf("Config written to %s\n", configPath)
	return nil
}

func printUsage() {
	fmt.Println(`skillsync — AI Agent Skills Installer

Usage:
  skillsync              Run interactive wizard
  skillsync list         List skills in registry
  skillsync status       Show installed skills per tool
  skillsync sync         Fetch/update remote bundles from Git
  skillsync uninstall    Remove a skill symlink
  skillsync init         Generate default config file
  skillsync help         Show this help

Flags:
  --config <path>        Use custom config file

Environment:
  SKILLSYNC_CONFIG       Config file path (default: ~/.config/skillsync/skillsync.yaml)`)
}
