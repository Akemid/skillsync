package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Akemid/skillsync/internal/config"
	"github.com/Akemid/skillsync/internal/installer"
	"github.com/Akemid/skillsync/internal/registry"
	"github.com/Akemid/skillsync/internal/sync"
	"github.com/Akemid/skillsync/internal/tui"
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
			Bundles:      config.DefaultBundles(),
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
				Bundles:      config.DefaultBundles(),
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
	if err := reg.Discover(cfg.Bundles...); err != nil {
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
		case "upgrade-config":
			return cmdUpgradeConfig(configPath)
		case "remote":
			return cmdRemote(cfg, configPath)
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
	result, err := tui.RunWizard(cfg, reg, projectDir, configPath)
	if err != nil {
		return err
	}
	if result == nil {
		// add-remote mode completed — no installation to do
		return nil
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

func cmdRemote(cfg *config.Config, configPath string) error {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  skillsync remote add <name> <url> [--branch <branch>] [--path <path>] [--company <company>]")
		fmt.Println("  skillsync remote list")
		return nil
	}

	switch os.Args[2] {
	case "list":
		return cmdRemoteList(cfg)
	case "add":
		return cmdRemoteAdd(cfg, configPath)
	default:
		return fmt.Errorf("unknown remote subcommand: %s", os.Args[2])
	}
}

func cmdRemoteList(cfg *config.Config) error {
	var remotes []config.Bundle
	for _, b := range cfg.Bundles {
		if b.Source != nil {
			remotes = append(remotes, b)
		}
	}
	if len(remotes) == 0 {
		fmt.Println("No remote bundles configured.")
		return nil
	}
	fmt.Printf("Remote bundles (%d):\n\n", len(remotes))
	for _, b := range remotes {
		company := ""
		if b.Company != "" {
			company = fmt.Sprintf(" [%s]", b.Company)
		}
		fmt.Printf("  %-20s%s\n", b.Name, company)
		fmt.Printf("    url:    %s\n", b.Source.URL)
		fmt.Printf("    branch: %s\n", b.Source.Branch)
		if b.Source.Path != "" {
			fmt.Printf("    path:   %s\n", b.Source.Path)
		}
	}
	return nil
}

func cmdRemoteAdd(cfg *config.Config, configPath string) error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: skillsync remote add <name> <url> [--branch <branch>] [--path <path>] [--company <company>]")
	}
	name := os.Args[3]
	gitURL := os.Args[4]
	branch := "main"
	path := ""
	company := ""

	args := os.Args[5:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--branch":
			if i+1 < len(args) {
				i++
				branch = args[i]
			}
		case "--path":
			if i+1 < len(args) {
				i++
				path = args[i]
			}
		case "--company":
			if i+1 < len(args) {
				i++
				company = args[i]
			}
		}
	}

	// Check for duplicates
	for _, b := range cfg.Bundles {
		if b.Name == name {
			return fmt.Errorf("bundle %q already exists in config", name)
		}
	}

	cfg.Bundles = append(cfg.Bundles, config.Bundle{
		Name:    name,
		Company: company,
		Source: &config.Source{
			Type:   "git",
			URL:    gitURL,
			Branch: branch,
			Path:   path,
		},
	})

	if err := config.Save(cfg, configPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("Bundle %q added to config (%s)\n", name, configPath)
	fmt.Printf("Run `skillsync sync` to fetch it.\n")
	return nil
}

func cmdInit(cfg *config.Config, configPath string) error {
	if _, err := os.Stat(configPath); err == nil {
		fmt.Fprintf(os.Stderr, "Warning: config already exists at %s; use `skillsync upgrade-config` to migrate without overwriting\n", configPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checking config path: %w", err)
	}

	if err := config.Save(cfg, configPath); err != nil {
		return err
	}
	fmt.Printf("Config written to %s\n", configPath)
	return nil
}

func cmdUpgradeConfig(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no config found at %s; run `skillsync init` first", configPath)
		}
		return fmt.Errorf("loading config from %s: %w", configPath, err)
	}

	migratedTools, summary := config.MigrateTools(cfg.Tools)
	cfg.Tools = migratedTools

	if err := config.Save(cfg, configPath); err != nil {
		return fmt.Errorf("saving upgraded config: %w", err)
	}

	fmt.Printf("Config upgraded at %s\n", configPath)
	if summary.MigratedLegacy {
		fmt.Println("- migrated legacy tool: kiro -> kiro-ide + kiro-cli")
	}
	if len(summary.AddedTools) > 0 {
		fmt.Printf("- added tools: %v\n", summary.AddedTools)
	}
	if summary.Unchanged {
		fmt.Println("- no changes required")
	}
	fmt.Println("- preserved bundles and registry_path")

	return nil
}

func printUsage() {
	fmt.Println(`skillsync — AI Agent Skills Installer

Usage:
  skillsync              	Run interactive wizard (install or add remote)
  skillsync list         	List skills in registry
  skillsync status       	Show installed skills per tool
  skillsync sync         	Fetch/update remote bundles from Git
  skillsync upgrade-config 	Migrate existing config safely
  skillsync remote list  	List configured remote bundles
  skillsync remote add  	Add a remote bundle to config
  skillsync uninstall   	Remove a skill symlink
  skillsync init        	Generate default config file
  skillsync help        	Show this help

Flags:
  --config <path>        Use custom config file

Environment:
  SKILLSYNC_CONFIG       Config file path (default: ~/.config/skillsync/skillsync.yaml)`)
}
