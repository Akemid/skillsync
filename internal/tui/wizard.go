package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Akemid/skillsync/internal/archive"
	"github.com/Akemid/skillsync/internal/config"
	"github.com/Akemid/skillsync/internal/detector"
	"github.com/Akemid/skillsync/internal/installer"
	"github.com/Akemid/skillsync/internal/registry"
	skillsync "github.com/Akemid/skillsync/internal/sync"
	"github.com/Akemid/skillsync/internal/tap"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// projectSkill represents a skill discovered in a project-local tool directory.
type projectSkill struct {
	Name     string
	Path     string
	ToolName string
}

// discoverProjectSkills scans each tool's LocalPath under projectDir and returns
// all skills found. Skills that are symlinks resolving into the central registry
// (regPath) are skipped. Results are deduplicated by local directory.
func discoverProjectSkills(projectDir string, tools []config.Tool, regPath string) []projectSkill {
	if projectDir == "" {
		return nil
	}

	registryExpanded := filepath.Clean(config.ExpandPath(regPath))
	registryAbs, _ := filepath.EvalSymlinks(registryExpanded)
	if registryAbs == "" {
		registryAbs = registryExpanded
	}
	seenLocalDirs := make(map[string]bool)
	var results []projectSkill

	for _, tool := range tools {
		if tool.LocalPath == "" {
			continue
		}
		absLocalDir := filepath.Join(projectDir, tool.LocalPath)
		absLocalDir = filepath.Clean(absLocalDir)

		// Deduplicate tools that share the same LocalPath
		if seenLocalDirs[absLocalDir] {
			continue
		}
		seenLocalDirs[absLocalDir] = true

		entries, err := os.ReadDir(absLocalDir)
		if err != nil {
			// Directory doesn't exist or not readable — skip
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
				continue
			}
			skillName := entry.Name()
			if strings.HasPrefix(skillName, ".") {
				continue
			}

			skillPath := filepath.Join(absLocalDir, skillName)

			// If it's a symlink, resolve it and skip if it points into the registry
			resolved, err := filepath.EvalSymlinks(skillPath)
			if err == nil {
				if strings.HasPrefix(filepath.Clean(resolved), registryAbs+string(filepath.Separator)) ||
					filepath.Clean(resolved) == registryAbs {
					continue
				}
				skillPath = resolved
			}

			// Verify SKILL.md exists
			if _, err := os.Stat(filepath.Join(skillPath, "SKILL.md")); err != nil {
				continue
			}

			results = append(results, projectSkill{
				Name:     skillName,
				Path:     skillPath,
				ToolName: tool.Name,
			})
		}
	}

	return results
}

const banner = `
   _____ __   _ _________                 
  / ___// /__(_) / / ___/__  ______  _____
  \__ \/ //_/ / / /\__ \/ / / / __ \/ ___/
 ___/ / ,< / / / /___/ / /_/ / / / / /__  
/____/_/|_/_/_/_//____/\__, /_/ /_/\___/  
                      /____/              `

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
	SkillsByBundle map[string][]string // skills grouped by bundle, for summary
	SelectedSkills []string            // flattened + deduplicated, for installation
	SelectedTools  []string
	Scope          installer.Scope
	ProjectDir     string
}

// newForm creates a themed form — DRY helper to avoid repeating WithTheme everywhere.
func newForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(huh.ThemeCatppuccin())
}

// RunWizard orchestrates the interactive TUI wizard step by step.
func RunWizard(cfg *config.Config, reg *registry.Registry, projectDir, configPath string) (*WizardResult, error) {
	fmt.Println(titleStyle.Render(banner))
	fmt.Println(dimStyle.Render("  Synchronize skills across your agentic coding tools\n"))

	mode, err := askWizardMode()
	if err != nil {
		return nil, err
	}

	switch mode {
	case "add-remote":
		return nil, runAddRemoteWizard(cfg, configPath)
	case "share-skill":
		return nil, runShareSkillWizard(cfg, reg, configPath, projectDir)
	case "export-skill":
		return nil, runExportWizard(cfg, reg, projectDir)
	case "import-skill":
		return nil, runImportWizard(cfg)
	}

	result := &WizardResult{ProjectDir: projectDir, SkillsByBundle: make(map[string][]string)}

	scope, scopeStr, err := askScope()
	if err != nil {
		return nil, err
	}
	result.Scope = scope

	// Step 1: pick one or more bundles
	selectedBundles, err := askBundles(cfg, reg)
	if err != nil {
		return nil, err
	}

	// Step 2: for each bundle, pick its skills
	skillsByBundle, err := pickSkillsPerBundle(cfg, reg, selectedBundles)
	if err != nil {
		return nil, err
	}
	result.SkillsByBundle = skillsByBundle
	result.SelectedSkills = flattenSkills(skillsByBundle)

	if len(result.SelectedSkills) == 0 {
		return nil, fmt.Errorf("no skills selected")
	}

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

// askWizardMode asks the user what they want to do.
func askWizardMode() (string, error) {
	var mode string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What do you want to do?").
				Options(
					huh.NewOption("Install skills", "install"),
					huh.NewOption("Add remote repository", "add-remote"),
					huh.NewOption("Share a skill (tap)", "share-skill"),
					huh.NewOption("Export skill to archive", "export-skill"),
					huh.NewOption("Import skill from archive", "import-skill"),
				).
				Value(&mode),
		),
	).Run()
	return mode, err
}

// runAddRemoteWizard guides the user through adding a new remote bundle to the config.
// It writes the bundle to the config file and offers to sync it immediately.
func runAddRemoteWizard(cfg *config.Config, configPath string) error {
	var name, url, branch, path, company string
	branch = "main"

	err := newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Bundle name").
				Description("Short identifier (e.g. company-skills)").
				Value(&name),
			huh.NewInput().
				Title("Git URL").
				Description("HTTPS or SSH clone URL").
				Value(&url),
			huh.NewInput().
				Title("Branch").
				Description("Branch to track (default: main)").
				Value(&branch),
			huh.NewInput().
				Title("Path (optional)").
				Description("Subdirectory inside the repo containing skills").
				Value(&path),
			huh.NewInput().
				Title("Company (optional)").
				Description("Company or team name (e.g. Acme)").
				Value(&company),
		),
	).Run()
	if err != nil {
		return err
	}

	if name == "" || url == "" {
		return fmt.Errorf("bundle name and URL are required")
	}

	src := &config.Source{
		Type:   "git",
		URL:    url,
		Branch: branch,
		Path:   path,
	}

	bundle := config.Bundle{
		Name:    name,
		Company: company,
		Source:  src,
	}

	cfg.Bundles = append(cfg.Bundles, bundle)

	fmt.Println(dimStyle.Render("\n  ⚠  This will overwrite your config file (comments and custom formatting will be lost)."))

	var confirmed bool
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Save bundle %q to %s?", name, configPath)).
				Value(&confirmed),
		),
	).Run()
	if err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("cancelled")
	}

	if err := config.Save(cfg, configPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Println(successStyle.Render(fmt.Sprintf("  ✓ Bundle %q saved to config", name)))

	var doSync bool
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Sync the bundle now?").
				Value(&doSync),
		),
	).Run()
	if err != nil {
		return err
	}

	if doSync {
		if err := downloadRemoteBundle(cfg, bundle); err != nil {
			return err
		}
	} else {
		fmt.Println(dimStyle.Render(fmt.Sprintf("  Run `skillsync sync` to fetch %q later.", name)))
	}

	return nil
}

// runShareSkillWizard guides the user through uploading a skill to a registered tap.
// If no taps are registered, it prompts to register one inline.
// projectDir is the current working directory; when non-empty, project-local skills
// are discovered and merged with registry skills in the selection list.
func runShareSkillWizard(cfg *config.Config, reg *registry.Registry, configPath, projectDir string) error {
	// If no taps, prompt to register one first
	if len(cfg.Taps) == 0 {
		fmt.Println(dimStyle.Render("  No taps registered. Let's add one first.\n"))

		var tapName, tapURL, tapBranch string
		tapBranch = "main"

		err := newForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Tap name").
					Description("Short identifier (e.g. my-skills)").
					Value(&tapName),
				huh.NewInput().
					Title("Git URL").
					Description("HTTPS or SSH clone URL of a writable repository").
					Value(&tapURL),
				huh.NewInput().
					Title("Branch (default: main)").
					Value(&tapBranch),
			),
		).Run()
		if err != nil {
			return err
		}
		if tapName == "" || tapURL == "" {
			return fmt.Errorf("tap name and URL are required")
		}

		cfg.Taps = append(cfg.Taps, config.Tap{Name: tapName, URL: tapURL, Branch: tapBranch})
		if err := config.Save(cfg, configPath); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Println(successStyle.Render(fmt.Sprintf("  ✓ Tap %q registered", tapName)))
	}

	// Build merged skill options (registry + project-local)
	regSkillNames := localBundleSkills(reg)
	regSkillsForMerge := make([]registry.Skill, 0, len(regSkillNames))
	for _, s := range reg.Skills {
		if !strings.Contains(s.Path, "/_remote/") {
			regSkillsForMerge = append(regSkillsForMerge, s)
		}
	}
	projectLocalSkills := discoverProjectSkills(projectDir, cfg.Tools, cfg.RegistryPath)
	skillOpts := buildMergedSkillOptions(regSkillsForMerge, projectLocalSkills)
	if len(skillOpts) == 0 {
		return fmt.Errorf("no local skills available to share")
	}

	// Build tap options
	tapOpts := make([]huh.Option[string], 0, len(cfg.Taps))
	for _, t := range cfg.Taps {
		tapOpts = append(tapOpts, huh.NewOption(fmt.Sprintf("%s (%s)", t.Name, t.URL), t.Name))
	}

	var selectedKey, selectedTap string
	force := false

	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select skill to share").
				Options(skillOpts...).
				Value(&selectedKey),
			huh.NewSelect[string]().
				Title("Select tap to upload to").
				Options(tapOpts...).
				Value(&selectedTap),
			huh.NewConfirm().
				Title("Overwrite if skill already exists in tap?").
				Value(&force),
		),
	).Run()
	if err != nil {
		return err
	}

	// Resolve skill path from selected key
	skillPath, err := resolveSkillPath(selectedKey, regSkillsForMerge, projectLocalSkills)
	if err != nil {
		return fmt.Errorf("resolving skill: %w", err)
	}

	// Extract skill name from the key
	parts := strings.SplitN(selectedKey, ":", 2)
	var selectedSkill string
	if parts[0] == "registry" {
		selectedSkill = parts[1]
	} else {
		selectedSkill = filepath.Base(skillPath)
	}

	// Find tap
	var foundTap config.Tap
	for _, t := range cfg.Taps {
		if t.Name == selectedTap {
			foundTap = t
			break
		}
	}

	tapper, err := tap.New(cfg.RegistryPath)
	if err != nil {
		return fmt.Errorf("initializing tapper: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := tapper.Upload(ctx, foundTap, skillPath, selectedSkill, force); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	fmt.Println(successStyle.Render(fmt.Sprintf("\n  ✓ Skill %q uploaded to tap %q", selectedSkill, selectedTap)))
	fmt.Println(dimStyle.Render("  Others can install it with:"))
	fmt.Printf("    skillsync remote add %s %s\n", selectedTap, foundTap.URL)
	fmt.Printf("    skillsync sync\n")
	return nil
}

// runExportWizard guides the user through exporting a local skill to a .tar.gz archive.
// cfg and projectDir are used to discover project-local skills alongside registry skills.
func runExportWizard(cfg *config.Config, reg *registry.Registry, projectDir string) error {
	// Build merged skill options (registry + project-local)
	regSkillsForMerge := make([]registry.Skill, 0)
	for _, s := range reg.Skills {
		if !strings.Contains(s.Path, "/_remote/") {
			regSkillsForMerge = append(regSkillsForMerge, s)
		}
	}
	projectLocalSkills := discoverProjectSkills(projectDir, cfg.Tools, cfg.RegistryPath)
	skillOpts := buildMergedSkillOptions(regSkillsForMerge, projectLocalSkills)
	if len(skillOpts) == 0 {
		return fmt.Errorf("no local skills available to export")
	}

	var selectedKey, outputPath string

	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select skill to export").
				Options(skillOpts...).
				Value(&selectedKey),
		),
	).Run()
	if err != nil {
		return err
	}

	// Resolve skill path from selected key
	skillPath, err := resolveSkillPath(selectedKey, regSkillsForMerge, projectLocalSkills)
	if err != nil {
		return fmt.Errorf("resolving skill: %w", err)
	}
	selectedSkill := filepath.Base(skillPath)

	outputPath = selectedSkill + ".tar.gz"

	err = newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Output path").
				Description("Path for the .tar.gz archive").
				Value(&outputPath),
		),
	).Run()
	if err != nil {
		return err
	}

	if err := archive.Export(skillPath, outputPath); err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	info, _ := os.Stat(outputPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	fmt.Println(successStyle.Render(fmt.Sprintf("\n  ✓ Exported %q → %s (%d bytes)", selectedSkill, outputPath, size)))
	return nil
}

// runImportWizard guides the user through importing a skill from a .tar.gz archive.
func runImportWizard(cfg *config.Config) error {
	var archivePath string

	err := newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Archive path").
				Description("Path to the .tar.gz file to import").
				Value(&archivePath),
		),
	).Run()
	if err != nil {
		return err
	}

	if archivePath == "" {
		return fmt.Errorf("archive path is required")
	}

	registryPath := config.ExpandPath(cfg.RegistryPath)

	// Preview: do a dry-run import to temp to show skill name
	tmpPreview, err := os.MkdirTemp("", ".skillsync-preview-*")
	if err != nil {
		return fmt.Errorf("creating preview temp dir: %w", err)
	}
	defer os.RemoveAll(tmpPreview)

	previewName, err := archive.Import(archivePath, tmpPreview, true)
	if err != nil {
		return fmt.Errorf("reading archive: %w", err)
	}

	fmt.Printf("\n  Skill found in archive: %s\n", previewName)

	var confirmed bool
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Install skill %q to %s?", previewName, registryPath)).
				Value(&confirmed),
		),
	).Run()
	if err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("import cancelled")
	}

	skillName, err := archive.Import(archivePath, registryPath, false)
	if err != nil {
		// Try with force if it exists
		var forceRetry bool
		if strings.Contains(err.Error(), "already installed") {
			err2 := newForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title(fmt.Sprintf("Skill %q already exists — overwrite?", previewName)).
						Value(&forceRetry),
				),
			).Run()
			if err2 != nil {
				return err2
			}
			if forceRetry {
				skillName, err = archive.Import(archivePath, registryPath, true)
				if err != nil {
					return fmt.Errorf("import failed: %w", err)
				}
			} else {
				return fmt.Errorf("import cancelled")
			}
		} else {
			return fmt.Errorf("import failed: %w", err)
		}
	}

	fmt.Println(successStyle.Render(fmt.Sprintf("\n  ✓ Skill %q installed to %s", skillName, filepath.Join(registryPath, skillName))))
	return nil
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

// localBundleKey is the virtual bundle identifier for skills in the local registry.
const localBundleKey = "__local__"

// askBundles shows a multi-select of all available bundles.
// A virtual "Local skills" entry (skills already in ~/.agents/skills) is always listed first.
func askBundles(cfg *config.Config, reg *registry.Registry) ([]string, error) {
	opts := make([]huh.Option[string], 0, len(cfg.Bundles)+1)

	localSkills := localBundleSkills(reg)
	localLabel := fmt.Sprintf("Local skills (%d available)", len(localSkills))
	opts = append(opts, huh.NewOption(localLabel, localBundleKey))

	for _, b := range cfg.Bundles {
		label := b.Name
		if b.Description != "" {
			desc := b.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			label = fmt.Sprintf("%s — %s", b.Name, desc)
		}
		opts = append(opts, huh.NewOption(label, b.Name))
	}

	var selected []string
	err := newForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select bundles to install from").
				Description("Space to select, Enter to confirm").
				Height(10).
				Options(opts...).
				Value(&selected),
		),
	).Run()
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no bundles selected")
	}
	return selected, nil
}

// pickSkillsPerBundle iterates over each selected bundle and prompts the user
// to pick skills from it. Returns a map of bundleName → selected skill names.
func pickSkillsPerBundle(cfg *config.Config, reg *registry.Registry, bundles []string) (map[string][]string, error) {
	result := make(map[string][]string, len(bundles))
	for _, bundleName := range bundles {
		var available []string
		var err error

		if bundleName == localBundleKey {
			available = localBundleSkills(reg)
		} else {
			available, err = resolveBundleSkills(cfg, bundleName, reg)
			if err != nil {
				return nil, err
			}
		}

		if len(available) == 0 {
			fmt.Println(dimStyle.Render(fmt.Sprintf("  No skills found in bundle %q — skipping", bundleName)))
			continue
		}

		selected, err := pickSkillsFromList(bundleName, available)
		if err != nil {
			return nil, err
		}
		if len(selected) > 0 {
			result[bundleName] = selected
		}
	}
	return result, nil
}

// pickSkillsFromList shows a multi-select for the given bundle with all skills pre-selected.
func pickSkillsFromList(bundleName string, skills []string) ([]string, error) {
	title := fmt.Sprintf("Skills from %q", bundleName)
	if bundleName == localBundleKey {
		title = "Local registry skills"
	}

	opts := make([]huh.Option[string], 0, len(skills))
	for _, name := range skills {
		opts = append(opts, huh.NewOption(name, name).Selected(true))
	}

	var selected []string
	err := newForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(title).
				Description("Space to deselect, Enter to confirm").
				Height(10).
				Options(opts...).
				Value(&selected),
		),
	).Run()

	return selected, err
}

// localBundleSkills returns the names of all skills in the local registry.
// localBundleSkills returns only skills from the local registry (not from _remote/ bundles).
func localBundleSkills(reg *registry.Registry) []string {
	names := make([]string, 0, len(reg.Skills))
	for _, s := range reg.Skills {
		if !strings.Contains(s.Path, "/_remote/") {
			names = append(names, s.Name)
		}
	}
	return names
}

// flattenSkills merges all skills from all bundles into a deduplicated slice.
func flattenSkills(byBundle map[string][]string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, skills := range byBundle {
		for _, name := range skills {
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}
	return result
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

// buildMergedSkillOptions builds a combined list of huh options from both
// central registry skills and project-local skills. Registry options use key
// "registry:<name>" and project-local options use key "local:<abs-path>".
func buildMergedSkillOptions(regSkills []registry.Skill, localSkills []projectSkill) []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(regSkills)+len(localSkills))

	for _, s := range regSkills {
		key := "registry:" + s.Name
		label := s.Name + " (registry)"
		opts = append(opts, huh.NewOption(label, key))
	}

	for _, s := range localSkills {
		key := "local:" + s.Path
		// Extract the tool dir name for the label: /proj/.claude/skills/my-skill → ".claude"
		// Path structure: <projectDir>/<toolLocalPath>/<skillName>
		// toolLocalPath examples: ".claude/skills", ".kiro/skills"
		// So Dir(Dir(Path)) = <projectDir>/<toolDir>, Base of that = .<toolName>
		toolDirName := filepath.Base(filepath.Dir(filepath.Dir(s.Path)))
		label := fmt.Sprintf("%s (%s)", s.Name, toolDirName)
		opts = append(opts, huh.NewOption(label, key))
	}

	return opts
}

// resolveSkillPath maps an option key produced by buildMergedSkillOptions back
// to the absolute path of the skill directory.
// Key format: "registry:<name>" or "local:<abs-path>"
func resolveSkillPath(key string, regSkills []registry.Skill, localSkills []projectSkill) (string, error) {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid skill key %q", key)
	}
	prefix, value := parts[0], parts[1]

	switch prefix {
	case "registry":
		for _, s := range regSkills {
			if s.Name == value {
				return s.Path, nil
			}
		}
		return "", fmt.Errorf("skill %q not found in registry", value)

	case "local":
		// value is the absolute path; look up in localSkills to verify it's known
		for _, s := range localSkills {
			if s.Path == value {
				return s.Path, nil
			}
		}
		return "", fmt.Errorf("local skill path %q not found", value)

	default:
		return "", fmt.Errorf("unknown skill key prefix %q in %q", prefix, key)
	}
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

// printSummary displays the installation summary grouped by bundle.
func printSummary(result *WizardResult, scopeStr string) {
	fmt.Println(headerStyle.Render("\n📋 Installation Summary"))
	for bundleName, skills := range result.SkillsByBundle {
		label := bundleName
		if bundleName == localBundleKey {
			label = "local"
		}
		fmt.Printf("  [%s]  %s\n", dimStyle.Render(label), strings.Join(skills, ", "))
	}
	fmt.Printf("  Tools:  %s\n", strings.Join(result.SelectedTools, ", "))
	fmt.Printf("  Scope:  %s\n", scopeStr)
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
	seen := make(map[string]bool)

	for i := range updated {
		globalPath := config.ExpandPath(updated[i].GlobalPath)
		if seen[globalPath] {
			continue
		}
		parentDir := strings.TrimSuffix(globalPath, "/skills")
		if _, err := os.Stat(parentDir); err == nil {
			updated[i].Enabled = true
			seen[globalPath] = true
		}
	}

	return updated
}
