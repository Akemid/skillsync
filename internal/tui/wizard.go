package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Akemid/skillsync/internal/config"
	"github.com/Akemid/skillsync/internal/detector"
	"github.com/Akemid/skillsync/internal/installer"
	"github.com/Akemid/skillsync/internal/registry"
	skillsync "github.com/Akemid/skillsync/internal/sync"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#3B82F6")).
			MarginTop(1)
)

// WizardResult holds the user's selections from the TUI wizard
type WizardResult struct {
	SelectedBundle string
	SelectedSkills []string
	SelectedTools  []string
	Scope          installer.Scope
	ProjectDir     string
}

// newForm creates a themed form — DRY helper to avoid repeating WithTheme everywhere.
func newForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(huh.ThemeCatppuccin())
}

// RunWizard orchestrates the interactive TUI wizard step by step.
func RunWizard(cfg *config.Config, reg *registry.Registry, projectDir string) (*WizardResult, error) {
	fmt.Println(titleStyle.Render("⚡ skillsync — AI Agent Skills Installer"))
	fmt.Println(dimStyle.Render("Synchronize skills across your agentic coding tools\n"))

	result := &WizardResult{ProjectDir: projectDir}

	scope, scopeStr, err := askScope()
	if err != nil {
		return nil, err
	}
	result.Scope = scope

	bundle, skills, err := askSkills(cfg, reg)
	if err != nil {
		return nil, err
	}
	result.SelectedBundle = bundle
	result.SelectedSkills = skills

	printDetectedTech(projectDir)

	tools, err := askTools(cfg)
	if err != nil {
		return nil, err
	}
	result.SelectedTools = tools

	printSummary(result, scopeStr)

	confirmed, err := askConfirm()
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return nil, fmt.Errorf("installation cancelled")
	}

	return result, nil
}

// askScope prompts the user to choose between global and project scope.
func askScope() (installer.Scope, string, error) {
	var scopeStr string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Installation scope").
				Description("Where should skills be installed?").
				Options(
					huh.NewOption("Global (home directory — available in all projects)", "global"),
					huh.NewOption("Project (current directory — project-specific)", "project"),
				).
				Value(&scopeStr),
		),
	).Run()
	if err != nil {
		return installer.ScopeGlobal, "", err
	}

	if scopeStr == "global" {
		return installer.ScopeGlobal, scopeStr, nil
	}
	return installer.ScopeProject, scopeStr, nil
}

// askSkills asks the user to select skills, either from a bundle or individually.
// Returns the selected bundle name (empty if individual), the skill names, and any error.
func askSkills(cfg *config.Config, reg *registry.Registry) (bundle string, skills []string, err error) {
	mode, err := askSelectionMode(cfg)
	if err != nil {
		return "", nil, err
	}

	if mode == "bundle" {
		bundle, skills, err = askBundleSkills(cfg, reg)
	} else {
		skills, err = askIndividualSkills(reg)
	}
	if err != nil {
		return "", nil, err
	}

	if len(skills) == 0 {
		return "", nil, fmt.Errorf("no skills selected")
	}

	return bundle, skills, nil
}

// askSelectionMode asks whether the user wants a bundle or individual skill selection.
// Returns "bundle" or "individual". Falls back to "individual" when no bundles are configured.
func askSelectionMode(cfg *config.Config) (string, error) {
	if len(cfg.Bundles) == 0 {
		return "individual", nil
	}

	var mode string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Skill selection").
				Description("How would you like to select skills?").
				Options(
					huh.NewOption("Choose a bundle (pre-configured skill set)", "bundle"),
					huh.NewOption("Pick individual skills from registry", "individual"),
				).
				Value(&mode),
		),
	).Run()

	return mode, err
}

// askBundleSkills lets the user pick a bundle, resolves its skills, then shows a
// confirmation multi-select so the user can tweak the suggestion before installing.
func askBundleSkills(cfg *config.Config, reg *registry.Registry) (bundle string, skills []string, err error) {
	err = newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select bundle").
				Options(buildBundleOptions(cfg.Bundles)...).
				Value(&bundle),
		),
	).Run()
	if err != nil {
		return "", nil, err
	}

	skills, err = resolveBundleSkills(cfg, bundle, reg)
	if err != nil {
		return "", nil, err
	}

	skills, err = askBundleConfirmation(skills)
	return bundle, skills, err
}

// askBundleConfirmation shows only the bundle's skills as a multi-select.
// The user can deselect individual skills before installing.
func askBundleConfirmation(preSelected []string) ([]string, error) {
	if len(preSelected) == 0 {
		return nil, nil
	}

	opts := make([]huh.Option[string], 0, len(preSelected))
	for _, name := range preSelected {
		opts = append(opts, huh.NewOption(name, name).Selected(true))
	}

	var selected []string
	err := newForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Confirm skills to install").
				Description("Space to deselect, Enter to confirm.").
				Height(10).
				Options(opts...).
				Value(&selected),
		),
	).Run()

	return selected, err
}

// buildBundleOptions converts bundle config into huh select options.
func buildBundleOptions(bundles []config.Bundle) []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(bundles))
	for _, b := range bundles {
		label := b.Name
		if b.Company != "" {
			label = fmt.Sprintf("%s (%s)", label, b.Company)
		}
		if b.Description != "" {
			label = fmt.Sprintf("%s — %s", label, b.Description)
		}
		opts = append(opts, huh.NewOption(label, b.Name))
	}
	return opts
}

// resolveBundleSkills returns the skill names for the given bundle.
// If the bundle has a remote source and hasn't been synced yet, it auto-syncs first.
func resolveBundleSkills(cfg *config.Config, bundleName string, reg *registry.Registry) ([]string, error) {
	for _, b := range cfg.Bundles {
		if b.Name != bundleName {
			continue
		}
		if len(b.Skills) > 0 {
			return explicitBundleSkills(b), nil
		}
		if b.Source != nil {
			return syncAndReadRemoteBundle(cfg, b, reg)
		}
		return nil, nil
	}
	return nil, nil
}

// explicitBundleSkills extracts skill names from a bundle's inline skill list.
func explicitBundleSkills(b config.Bundle) []string {
	names := make([]string, 0, len(b.Skills))
	for _, sr := range b.Skills {
		names = append(names, sr.Name)
	}
	return names
}

// syncAndReadRemoteBundle ensures the bundle is synced locally, then returns its skill names.
// After a fresh clone it re-runs Discover so the registry reflects the new skills.
func syncAndReadRemoteBundle(cfg *config.Config, b config.Bundle, reg *registry.Registry) ([]string, error) {
	registryPath := config.ExpandPath(cfg.RegistryPath)
	remoteBundleDir := filepath.Join(registryPath, "_remote", b.Name)
	if b.Source.Path != "" {
		remoteBundleDir = filepath.Join(remoteBundleDir, b.Source.Path)
	}

	if _, err := os.Stat(remoteBundleDir); os.IsNotExist(err) {
		if err := downloadRemoteBundle(cfg, b); err != nil {
			return nil, err
		}
		// Refresh registry so the newly cloned skills appear in the confirmation menu
		if err := reg.Discover(cfg.Bundles...); err != nil {
			return nil, fmt.Errorf("refreshing registry after sync: %w", err)
		}
	}

	return readSkillsFromDir(remoteBundleDir, b.Name)
}

// downloadRemoteBundle clones or pulls a remote bundle into the local registry.
func downloadRemoteBundle(cfg *config.Config, b config.Bundle) error {
	fmt.Println(dimStyle.Render(fmt.Sprintf("  Bundle %q not synced — downloading from %s...", b.Name, b.Source.URL)))

	registryAbs := config.ExpandPath(cfg.RegistryPath)
	syncer, err := skillsync.New(filepath.Join(registryAbs, "_remote"))
	if err != nil {
		return fmt.Errorf("initializing syncer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := syncer.SyncBundle(ctx, b.Name, b.Source.URL, b.Source.Branch); err != nil {
		return fmt.Errorf("auto-sync failed for bundle %q: %w\n\nRun manually: skillsync sync", b.Name, err)
	}

	fmt.Println(successStyle.Render(fmt.Sprintf("  ✓ %s synced", b.Name)))
	return nil
}

// readSkillsFromDir returns the names of all non-hidden subdirectories in dir.
func readSkillsFromDir(dir, bundleName string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading remote bundle %q: %w", bundleName, err)
	}

	var skills []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			skills = append(skills, entry.Name())
		}
	}
	return skills, nil
}

// askIndividualSkills shows a multi-select with all available registry skills.
func askIndividualSkills(reg *registry.Registry) ([]string, error) {
	opts := buildSkillOptions(reg.Skills)
	if len(opts) == 0 {
		return nil, fmt.Errorf("no skills found in registry at %s", reg.BasePath)
	}

	var selected []string
	err := newForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select skills to install").
				Description("Space to select, Enter to confirm").
				Height(10).
				Options(opts...).
				Value(&selected),
		),
	).Run()

	return selected, err
}

// buildSkillOptions converts registry skills into huh multi-select options.
// Descriptions are truncated to 60 chars to avoid wrapping in narrow terminals.
func buildSkillOptions(skills []registry.Skill) []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(skills))
	for _, s := range skills {
		label := s.Name
		if s.Description != "" {
			desc := s.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			label = fmt.Sprintf("%s — %s", s.Name, desc)
		}
		opts = append(opts, huh.NewOption(label, s.Name))
	}
	return opts
}

// printDetectedTech prints detected technologies for the given project directory.
func printDetectedTech(projectDir string) {
	if projectDir == "" {
		return
	}
	techs := detector.Detect(projectDir)
	if len(techs) > 0 {
		fmt.Println(dimStyle.Render(fmt.Sprintf("  Detected tech: %s", strings.Join(techs, ", "))))
	}
}

// askTools shows a multi-select with all configured tools, pre-selecting detected ones.
func askTools(cfg *config.Config) ([]string, error) {
	opts, preSelected := buildToolOptions(cfg.Tools)

	selected := preSelected
	err := newForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Target agentic tools").
				Description("Which tools should receive these skills?").
				Options(opts...).
				Value(&selected),
		),
	).Run()
	if err != nil {
		return nil, err
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no tools selected")
	}
	return selected, nil
}

// buildToolOptions converts tool config into huh multi-select options and returns pre-selected names.
func buildToolOptions(tools []config.Tool) (opts []huh.Option[string], preSelected []string) {
	opts = make([]huh.Option[string], 0, len(tools))
	for _, t := range tools {
		label := t.Name
		if t.Enabled {
			label = fmt.Sprintf("%s (detected)", t.Name)
			preSelected = append(preSelected, t.Name)
		}
		opts = append(opts, huh.NewOption(label, t.Name))
	}
	return opts, preSelected
}

// printSummary displays the installation summary before the confirmation prompt.
func printSummary(result *WizardResult, scopeStr string) {
	fmt.Println(headerStyle.Render("\n📋 Installation Summary"))
	fmt.Printf("  Skills:  %s\n", strings.Join(result.SelectedSkills, ", "))
	fmt.Printf("  Tools:   %s\n", strings.Join(result.SelectedTools, ", "))
	fmt.Printf("  Scope:   %s\n", scopeStr)
	if result.SelectedBundle != "" {
		fmt.Printf("  Bundle:  %s\n", result.SelectedBundle)
	}
	fmt.Println()
}

// askConfirm asks the user to confirm the installation.
func askConfirm() (bool, error) {
	var confirm bool
	err := newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Proceed with installation?").
				Value(&confirm),
		),
	).Run()
	return confirm, err
}

// PrintResults displays the installation results
func PrintResults(results []installer.Result) {
	fmt.Println(headerStyle.Render("\n📦 Installation Results"))

	created, existed, errors := 0, 0, 0
	for _, r := range results {
		switch {
		case r.Error != nil:
			fmt.Printf("  %s %s → %s: %s\n",
				errorStyle.Render("✗"),
				r.Skill, r.Tool,
				errorStyle.Render(r.Error.Error()))
			errors++
		case r.Existed:
			fmt.Printf("  %s %s → %s %s\n",
				dimStyle.Render("○"),
				r.Skill, r.Tool,
				dimStyle.Render("(already exists)"))
			existed++
		case r.Created:
			fmt.Printf("  %s %s → %s\n",
				successStyle.Render("✓"),
				r.Skill, r.Tool)
			created++
		}
	}

	fmt.Printf("\n  %s  %s  %s\n",
		successStyle.Render(fmt.Sprintf("%d created", created)),
		dimStyle.Render(fmt.Sprintf("%d existing", existed)),
		errorStyle.Render(fmt.Sprintf("%d errors", errors)))
}

// DetectInstalledTools checks which tools have their directories present
func DetectInstalledTools(tools []config.Tool) []config.Tool {
	updated := make([]config.Tool, len(tools))
	copy(updated, tools)

	for i := range updated {
		globalPath := config.ExpandPath(updated[i].GlobalPath)
		parentDir := strings.TrimSuffix(globalPath, "/skills")
		if _, err := os.Stat(parentDir); err == nil {
			updated[i].Enabled = true
		}
	}

	return updated
}
