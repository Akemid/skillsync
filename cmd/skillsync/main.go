package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Akemid/skillsync/internal/archive"
	"github.com/Akemid/skillsync/internal/config"
	"github.com/Akemid/skillsync/internal/installer"
	"github.com/Akemid/skillsync/internal/registry"
	"github.com/Akemid/skillsync/internal/skillasset"
	"github.com/Akemid/skillsync/internal/sync"
	"github.com/Akemid/skillsync/internal/tap"
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

	// Handle self-skill subcommand early (after config load, before registry scan)
	if len(os.Args) > 2 && os.Args[1] == "self-skill" {
		switch os.Args[2] {
		case "install":
			yesFlag := false
			for _, arg := range os.Args[3:] {
				if arg == "--yes" {
					yesFlag = true
				}
			}
			return cmdSelfSkillInstall(cfg, yesFlag)
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: self-skill %s\n\n", os.Args[2])
			printUsage()
			return nil
		}
	}

	// Auto-detect which tools are installed
	cfg.Tools = tui.DetectInstalledTools(cfg.Tools)

	// Initialize registry
	reg := registry.New(cfg.RegistryPath)
	if err := reg.Discover(cfg.Bundles...); err != nil {
		return fmt.Errorf("scanning skill registry: %w", err)
	}

	// Skip the empty-registry guard for "install": cmdInstall re-discovers after auto-sync,
	// so it must be allowed to run even when the registry is empty (REQ-18).
	isInstall := len(os.Args) > 1 && os.Args[1] == "install"
	if !isInstall && len(reg.Skills) == 0 {
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
		case "install":
			return cmdInstall(cfg, reg, projectDir)
		case "uninstall":
			return cmdUninstall(cfg, reg, projectDir)
		case "tap":
			return cmdTap(cfg, configPath)
		case "upload":
			return cmdUpload(cfg, reg, configPath)
		case "export":
			return cmdExport(reg)
		case "import":
			return cmdImport(cfg, configPath)
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

	if result.SelfSkillRequested {
		return installSelfSkill(cfg, cfg.Tools, true)
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

	return installSelfSkill(cfg, selectedTools, true)
}

// cmdSelfSkillInstall handles `skillsync self-skill install [--yes]`.
// yesFlag skips the interactive confirmation prompt.
func cmdSelfSkillInstall(cfg *config.Config, yesFlag bool) error {
	return installSelfSkill(cfg, tui.DetectInstalledTools(cfg.Tools), !yesFlag)
}

// installSelfSkill extracts the embedded skillsync skill to the registry and
// symlinks it to all provided tools at global scope.
// If the skill is already installed with matching content it prints a notice and returns nil.
// If interactive is true the user is prompted before installation proceeds.
func installSelfSkill(cfg *config.Config, tools []config.Tool, interactive bool) error {
	registryPath := config.ExpandPath(cfg.RegistryPath)
	selfSkillDir := filepath.Join(registryPath, skillasset.SkillName)
	selfSkillMD := filepath.Join(selfSkillDir, "SKILL.md")

	if existing, err := os.ReadFile(selfSkillMD); err == nil {
		if bytes.Equal(existing, skillasset.Content()) {
			fmt.Println("skillsync skill already installed")
			return nil
		}
	}

	if interactive {
		if !tui.ConfirmSelfSkillInstall() {
			return nil
		}
	}

	if err := skillasset.ExtractTo(registryPath); err != nil {
		return fmt.Errorf("extracting self-skill: %w", err)
	}

	selfSkill := registry.Skill{Name: skillasset.SkillName, Path: selfSkillDir}
	results := installer.Install([]registry.Skill{selfSkill}, tools, installer.ScopeGlobal, "")
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
		if err := syncer.SyncBundle(ctx, bundle.Name, bundle.Source.URL, bundle.Source.Branch, bundle.SSHKey); err != nil {
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

// cmdTap dispatches tap subcommands: add, list, remove.
func cmdTap(cfg *config.Config, configPath string) error {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  skillsync tap add <name> <url> [--branch <branch>]")
		fmt.Println("  skillsync tap list")
		fmt.Println("  skillsync tap remove <name>")
		return nil
	}
	switch os.Args[2] {
	case "add":
		return cmdTapAdd(cfg, configPath)
	case "list":
		return cmdTapList(cfg)
	case "remove":
		return cmdTapRemove(cfg, configPath)
	default:
		return fmt.Errorf("unknown tap subcommand: %s", os.Args[2])
	}
}

func cmdTapAdd(cfg *config.Config, configPath string) error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: skillsync tap add <name> <url> [--branch <branch>]")
	}
	name := os.Args[3]
	url := os.Args[4]
	branch := "main"

	for i := 5; i < len(os.Args); i++ {
		if os.Args[i] == "--branch" && i+1 < len(os.Args) {
			i++
			branch = os.Args[i]
		}
	}

	// Check for duplicate tap name
	for _, t := range cfg.Taps {
		if t.Name == name {
			return fmt.Errorf("tap %q already registered", name)
		}
	}

	cfg.Taps = append(cfg.Taps, config.Tap{Name: name, URL: url, Branch: branch})
	if err := config.Save(cfg, configPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("Tap %q added (url: %s, branch: %s)\n", name, url, branch)
	return nil
}

func cmdTapList(cfg *config.Config) error {
	if len(cfg.Taps) == 0 {
		fmt.Println("No taps registered. Use `skillsync tap add` to register one.")
		return nil
	}
	fmt.Printf("Registered taps (%d):\n\n", len(cfg.Taps))
	for _, t := range cfg.Taps {
		fmt.Printf("  %-20s  %s\n", t.Name, t.URL)
	}
	return nil
}

func cmdTapRemove(cfg *config.Config, configPath string) error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: skillsync tap remove <name>")
	}
	name := os.Args[3]

	idx := -1
	for i, t := range cfg.Taps {
		if t.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("tap %q not found", name)
	}

	cfg.Taps = append(cfg.Taps[:idx], cfg.Taps[idx+1:]...)
	if err := config.Save(cfg, configPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("Tap %q removed\n", name)
	return nil
}

// cmdUpload uploads a local skill to a registered tap.
// Usage: skillsync upload <skill> --to <tap-name> [--force]
func cmdUpload(cfg *config.Config, reg *registry.Registry, configPath string) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: skillsync upload <skill> --to <tap-name> [--force]")
	}
	skillName := os.Args[2]
	tapName := ""
	force := false

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--to":
			if i+1 < len(os.Args) {
				i++
				tapName = os.Args[i]
			}
		case "--force":
			force = true
		}
	}

	if tapName == "" {
		return fmt.Errorf("--to <tap-name> is required")
	}

	// Resolve skill
	var foundSkill *registry.Skill
	for i := range reg.Skills {
		if reg.Skills[i].Name == skillName {
			foundSkill = &reg.Skills[i]
			break
		}
	}
	if foundSkill == nil {
		return fmt.Errorf("skill %q not found in registry", skillName)
	}

	// Resolve tap
	var foundTap *config.Tap
	for i := range cfg.Taps {
		if cfg.Taps[i].Name == tapName {
			foundTap = &cfg.Taps[i]
			break
		}
	}
	if foundTap == nil {
		return fmt.Errorf("tap %q not found; use `skillsync tap add` to register it", tapName)
	}

	tapper, err := tap.New(cfg.RegistryPath)
	if err != nil {
		return fmt.Errorf("initializing tapper: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := tapper.Upload(ctx, *foundTap, foundSkill.Path, foundSkill.Name, force); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	fmt.Printf("✓ Uploaded %q to tap %q.\n", skillName, tapName)
	fmt.Printf("Others can install it with:\n")
	fmt.Printf("  skillsync remote add %s %s\n", tapName, foundTap.URL)
	fmt.Printf("  skillsync sync\n")
	return nil
}

// cmdExport exports a skill to a .tar.gz archive.
// Usage: skillsync export <skill> [--output <path>]
func cmdExport(reg *registry.Registry) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: skillsync export <skill> [--output <path>]")
	}
	skillName := os.Args[2]
	outputPath := ""

	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--output" && i+1 < len(os.Args) {
			i++
			outputPath = os.Args[i]
		}
	}

	// Resolve skill
	var foundSkill *registry.Skill
	for i := range reg.Skills {
		if reg.Skills[i].Name == skillName {
			foundSkill = &reg.Skills[i]
			break
		}
	}
	if foundSkill == nil {
		return fmt.Errorf("skill %q not found in registry", skillName)
	}

	if outputPath == "" {
		outputPath = skillName + ".tar.gz"
	}

	if err := archive.Export(foundSkill.Path, outputPath); err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return fmt.Errorf("stat output: %w", err)
	}
	fmt.Printf("✓ Exported %q to %s (%d bytes)\n", skillName, outputPath, info.Size())
	return nil
}

// cmdImport imports a skill from a .tar.gz archive into the registry.
// Usage: skillsync import <file.tar.gz> [--force]
func cmdImport(cfg *config.Config, configPath string) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: skillsync import <file.tar.gz> [--force]")
	}
	archivePath := os.Args[2]
	force := false

	for _, arg := range os.Args[3:] {
		if arg == "--force" {
			force = true
		}
	}

	registryPath := config.ExpandPath(cfg.RegistryPath)

	skillName, err := archive.Import(archivePath, registryPath, force)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Printf("✓ Skill %q installed to %s\n", skillName, filepath.Join(registryPath, skillName))

	// Print description from SKILL.md if available
	skillMDPath := filepath.Join(registryPath, skillName, "SKILL.md")
	if desc := readSkillDescription(skillMDPath); desc != "" {
		fmt.Printf("Description: %s\n", desc)
	}

	return nil
}

// readSkillDescription reads the description field from a SKILL.md frontmatter.
// Returns an empty string if the file can't be read or the field is absent.
func readSkillDescription(skillMDPath string) string {
	data, err := os.ReadFile(skillMDPath)
	if err != nil {
		return ""
	}
	content := string(data)
	// Parse YAML frontmatter between --- delimiters
	if !hasPrefix(content, "---") {
		return ""
	}
	// Find closing ---
	rest := content[3:]
	end := findFrontmatterEnd(rest)
	if end < 0 {
		return ""
	}
	frontmatter := rest[:end]
	return extractYAMLField(frontmatter, "description")
}

// hasPrefix checks if s starts with prefix (avoids importing strings in main).
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// findFrontmatterEnd returns the index of the closing "---" line in s, or -1.
func findFrontmatterEnd(s string) int {
	i := 0
	for i < len(s) {
		nl := indexByte(s[i:], '\n')
		var line string
		if nl < 0 {
			line = s[i:]
			i = len(s)
		} else {
			line = s[i : i+nl]
			i += nl + 1
		}
		// Trim carriage return
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if line == "---" {
			return i - nl - 1 - 1 // position before this line
		}
	}
	return -1
}

// indexByte returns index of first occurrence of b in s, or -1.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// extractYAMLField extracts a simple "key: value" field from a YAML frontmatter string.
func extractYAMLField(frontmatter, key string) string {
	prefix := key + ":"
	lines := splitLines(frontmatter)
	for _, line := range lines {
		trimmed := trimLeft(line)
		if hasPrefix(trimmed, prefix) {
			val := trimLeft(trimmed[len(prefix):])
			// Remove optional surrounding quotes
			if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
				val = val[1 : len(val)-1]
			}
			return val
		}
	}
	return ""
}

// splitLines splits s into lines (handles \n and \r\n).
func splitLines(s string) []string {
	var lines []string
	for {
		nl := indexByte(s, '\n')
		if nl < 0 {
			if s != "" {
				lines = append(lines, s)
			}
			break
		}
		line := s[:nl]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
		s = s[nl+1:]
	}
	return lines
}

// trimLeft removes leading spaces and tabs from s.
func trimLeft(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[i:]
}

// cmdInstall handles `skillsync install --skill <ref> [--bundle <name>] [--tool <name>] [--scope global|project] [--yes]`.
// It is non-interactive: no TUI wizard is shown. Skills are resolved from the registry and installed directly.
func cmdInstall(cfg *config.Config, reg *registry.Registry, projectDir string) error {
	// Step 1: Parse flags from os.Args[2:]
	args := os.Args[2:]
	var skillFlags []string
	var bundleFlag string
	var toolFlags []string
	scopeFlag := "global"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--skill":
			if i+1 < len(args) {
				i++
				skillFlags = append(skillFlags, args[i])
			}
		case "--bundle":
			if i+1 < len(args) {
				i++
				bundleFlag = args[i]
			}
		case "--tool":
			if i+1 < len(args) {
				i++
				toolFlags = append(toolFlags, args[i])
			}
		case "--scope":
			if i+1 < len(args) {
				i++
				scopeFlag = args[i]
			}
		case "--yes":
			// no-op: forward compatibility
		}
	}

	// Step 2: Validate at least one skill or bundle provided
	if len(skillFlags) == 0 && bundleFlag == "" {
		return fmt.Errorf("usage: skillsync install --skill <name> [--bundle <bundle>] [--tool <tool>] [--scope global|project]")
	}

	// Step 3: Validate scope (before any I/O)
	if scopeFlag != "global" && scopeFlag != "project" {
		return fmt.Errorf("invalid scope %q: must be \"global\" or \"project\"", scopeFlag)
	}

	// Step 4: Validate tools (before any I/O)
	var resolvedTools []config.Tool
	if len(toolFlags) > 0 {
		for _, name := range toolFlags {
			found := false
			for _, t := range cfg.Tools {
				if t.Name == name {
					resolvedTools = append(resolvedTools, t)
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("unknown tool %q", name)
			}
		}
	} else {
		resolvedTools = tui.DetectInstalledTools(cfg.Tools)
	}

	// Step 5: Collect bundle names that may need syncing
	var bundlesToSync []string
	for _, s := range skillFlags {
		b, _ := parseSkillRef(s)
		if b != "" {
			bundlesToSync = append(bundlesToSync, b)
		}
	}
	if bundleFlag != "" {
		bundlesToSync = append(bundlesToSync, bundleFlag)
	}

	// Step 6: Auto-sync remote bundles if needed
	if err := syncRemoteBundlesIfNeeded(cfg, bundlesToSync); err != nil {
		return err
	}

	// Step 7: Re-discover registry (after any sync)
	if err := reg.Discover(cfg.Bundles...); err != nil {
		return fmt.Errorf("scanning skill registry: %w", err)
	}

	// Dedup skills by Path
	seenPaths := make(map[string]bool)
	var resolvedSkills []registry.Skill

	addSkill := func(s registry.Skill) {
		if !seenPaths[s.Path] {
			seenPaths[s.Path] = true
			resolvedSkills = append(resolvedSkills, s)
		}
	}

	// Step 8: Resolve --bundle flag
	if bundleFlag != "" {
		// Validate it's configured
		bundleConfigured := false
		for _, b := range cfg.Bundles {
			if b.Name == bundleFlag {
				bundleConfigured = true
				break
			}
		}
		if !bundleConfigured {
			return fmt.Errorf("bundle %q not configured", bundleFlag)
		}

		bundleSkills := reg.FindByBundle(bundleFlag)
		for _, s := range bundleSkills {
			addSkill(s)
		}
	}

	// Step 9: Resolve each --skill flag
	for _, s := range skillFlags {
		bundle, name := parseSkillRef(s)
		if bundle != "" {
			skill, ok := reg.FindByBundleAndName(bundle, name)
			if !ok {
				return fmt.Errorf("skill %q not found in bundle %q", name, bundle)
			}
			addSkill(skill)
		} else {
			// Plain name: collect all skills with this name
			var matches []registry.Skill
			for _, sk := range reg.Skills {
				if sk.Name == name {
					matches = append(matches, sk)
				}
			}
			switch len(matches) {
			case 0:
				return fmt.Errorf("skill %q not found in registry", name)
			case 1:
				addSkill(matches[0])
			default:
				// Ambiguous: list bundles and instruct bundle:skill syntax
				var bundles []string
				for _, m := range matches {
					if m.Bundle != "" {
						bundles = append(bundles, m.Bundle)
					} else {
						bundles = append(bundles, "(local)")
					}
				}
				return fmt.Errorf("skill %q is ambiguous — found in bundles: %s\nuse bundle:skill syntax (e.g. %s:%s)",
					name, strings.Join(bundles, ", "), bundles[0], name)
			}
		}
	}

	// Step 10: Map scope string to installer.Scope
	var scope installer.Scope
	if scopeFlag == "project" {
		scope = installer.ScopeProject
	} else {
		scope = installer.ScopeGlobal
	}

	// Step 11: Install
	results := installer.Install(resolvedSkills, resolvedTools, scope, projectDir)

	// Step 12: Print results
	tui.PrintResults(results)

	return nil
}

// syncRemoteBundlesIfNeeded auto-syncs any remote bundles that haven't been cloned yet.
// bundleNames is the list of bundle names referenced by the install command.
// Bundles without a Source (local-only) are skipped. Bundles whose _remote/{name}/ dir
// already exists are also skipped (already synced).
func syncRemoteBundlesIfNeeded(cfg *config.Config, bundleNames []string) error {
	// Build map of remote bundles (those with Source != nil)
	bundleByName := make(map[string]config.Bundle)
	for _, b := range cfg.Bundles {
		if b.Source != nil {
			bundleByName[b.Name] = b
		}
	}

	registryAbs := config.ExpandPath(cfg.RegistryPath)
	remoteBase := filepath.Join(registryAbs, "_remote")

	for _, name := range bundleNames {
		b, ok := bundleByName[name]
		if !ok {
			// Local bundle — no sync needed
			continue
		}

		remoteBundleDir := filepath.Join(remoteBase, name)
		if b.Source.Path != "" {
			remoteBundleDir = filepath.Join(remoteBundleDir, b.Source.Path)
		}

		if _, err := os.Stat(remoteBundleDir); err == nil {
			// Already synced
			continue
		}

		fmt.Printf("  Syncing %s from %s...\n", name, b.Source.URL)

		syncer, err := sync.New(remoteBase)
		if err != nil {
			return fmt.Errorf("auto-sync failed for bundle %q: %w", name, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		syncErr := syncer.SyncBundle(ctx, name, b.Source.URL, b.Source.Branch, b.SSHKey)
		cancel()

		if syncErr != nil {
			return fmt.Errorf("auto-sync failed for bundle %q: %w", name, syncErr)
		}

		fmt.Printf("  ✓ %s synced\n", name)
	}

	return nil
}

// parseSkillRef splits a skill reference into (bundle, name).
// "my-skill" → ("", "my-skill")
// "acme:go-testing" → ("acme", "go-testing")
// "a:b:c" → ("a", "b:c")
// "" → ("", "")
func parseSkillRef(s string) (bundle, name string) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
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
  skillsync install    	Install skills non-interactively
  skillsync uninstall   	Remove a skill symlink
  skillsync init        	Generate default config file
  skillsync tap add      	Register a writable git repo (tap) for uploading skills
  skillsync tap list     	List registered taps
  skillsync tap remove   	Remove a registered tap
  skillsync upload       	Upload a local skill to a registered tap
  skillsync export       	Export a skill to a .tar.gz archive
  skillsync import       	Import a skill from a .tar.gz archive
  skillsync self-skill install	Install the skillsync skill into your registry
  skillsync help        	Show this help

Flags:
  --config <path>        Use custom config file

Environment:
  SKILLSYNC_CONFIG       Config file path (default: ~/.config/skillsync/skillsync.yaml)`)
}
