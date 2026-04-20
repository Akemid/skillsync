package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Akemid/skillsync/internal/config"
	"github.com/Akemid/skillsync/internal/detector"
	"github.com/Akemid/skillsync/internal/installer"
	"github.com/Akemid/skillsync/internal/registry"
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

// RunWizard runs the interactive TUI wizard
func RunWizard(cfg *config.Config, reg *registry.Registry, projectDir string) (*WizardResult, error) {
	fmt.Println(titleStyle.Render("⚡ skillsync — AI Agent Skills Installer"))
	fmt.Println(dimStyle.Render("Synchronize skills across your agentic coding tools\n"))

	result := &WizardResult{ProjectDir: projectDir}

	// Step 1: Choose scope
	var scopeStr string
	scopeForm := huh.NewForm(
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
	).WithTheme(huh.ThemeCatppuccin())

	if err := scopeForm.Run(); err != nil {
		return nil, err
	}

	if scopeStr == "global" {
		result.Scope = installer.ScopeGlobal
	} else {
		result.Scope = installer.ScopeProject
	}

	// Step 2: Choose bundle or individual skills
	var selectionMode string
	if len(cfg.Bundles) > 0 {
		modeForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Skill selection").
					Description("How would you like to select skills?").
					Options(
						huh.NewOption("Choose a bundle (pre-configured skill set)", "bundle"),
						huh.NewOption("Pick individual skills from registry", "individual"),
					).
					Value(&selectionMode),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := modeForm.Run(); err != nil {
			return nil, err
		}
	} else {
		selectionMode = "individual"
	}

	if selectionMode == "bundle" {
		// Step 2a: Choose bundle
		bundleOptions := make([]huh.Option[string], 0, len(cfg.Bundles))
		for _, b := range cfg.Bundles {
			label := b.Name
			if b.Company != "" {
				label = fmt.Sprintf("%s (%s)", b.Name, b.Company)
			}
			if b.Description != "" {
				label = fmt.Sprintf("%s — %s", label, b.Description)
			}
			bundleOptions = append(bundleOptions, huh.NewOption(label, b.Name))
		}

		bundleForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select bundle").
					Options(bundleOptions...).
					Value(&result.SelectedBundle),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := bundleForm.Run(); err != nil {
			return nil, err
		}

		// Resolve bundle skills
		for _, b := range cfg.Bundles {
			if b.Name == result.SelectedBundle {
				for _, sr := range b.Skills {
					result.SelectedSkills = append(result.SelectedSkills, sr.Name)
				}
				break
			}
		}
	} else {
		// Step 2b: Pick individual skills
		skillOptions := make([]huh.Option[string], 0, len(reg.Skills))
		for _, s := range reg.Skills {
			label := s.Name
			if s.Description != "" {
				label = fmt.Sprintf("%s — %s", s.Name, s.Description)
			}
			skillOptions = append(skillOptions, huh.NewOption(label, s.Name))
		}

		if len(skillOptions) == 0 {
			return nil, fmt.Errorf("no skills found in registry at %s", reg.BasePath)
		}

		skillForm := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select skills to install").
					Description("Space to select, Enter to confirm").
					Options(skillOptions...).
					Value(&result.SelectedSkills),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := skillForm.Run(); err != nil {
			return nil, err
		}
	}

	if len(result.SelectedSkills) == 0 {
		return nil, fmt.Errorf("no skills selected")
	}

	// Step 3: Auto-detect tech and show it
	if projectDir != "" {
		techs := detector.Detect(projectDir)
		if len(techs) > 0 {
			fmt.Println(dimStyle.Render(fmt.Sprintf("  Detected tech: %s", strings.Join(techs, ", "))))
		}
	}

	// Step 4: Choose target tools
	toolOptions := make([]huh.Option[string], 0, len(cfg.Tools))
	for _, t := range cfg.Tools {
		label := t.Name
		if t.Enabled {
			label = fmt.Sprintf("%s (detected)", t.Name)
		}
		toolOptions = append(toolOptions, huh.NewOption(label, t.Name))
	}

	// Pre-select enabled tools
	var preSelected []string
	for _, t := range cfg.Tools {
		if t.Enabled {
			preSelected = append(preSelected, t.Name)
		}
	}
	result.SelectedTools = preSelected

	toolForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Target agentic tools").
				Description("Which tools should receive these skills?").
				Options(toolOptions...).
				Value(&result.SelectedTools),
		),
	).WithTheme(huh.ThemeCatppuccin())

	if err := toolForm.Run(); err != nil {
		return nil, err
	}

	if len(result.SelectedTools) == 0 {
		return nil, fmt.Errorf("no tools selected")
	}

	// Step 5: Confirm
	fmt.Println(headerStyle.Render("\n📋 Installation Summary"))
	fmt.Printf("  Skills:  %s\n", strings.Join(result.SelectedSkills, ", "))
	fmt.Printf("  Tools:   %s\n", strings.Join(result.SelectedTools, ", "))
	fmt.Printf("  Scope:   %s\n", scopeStr)
	if result.SelectedBundle != "" {
		fmt.Printf("  Bundle:  %s\n", result.SelectedBundle)
	}
	fmt.Println()

	var confirm bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Proceed with installation?").
				Value(&confirm),
		),
	).WithTheme(huh.ThemeCatppuccin())

	if err := confirmForm.Run(); err != nil {
		return nil, err
	}

	if !confirm {
		return nil, fmt.Errorf("installation cancelled")
	}

	return result, nil
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
