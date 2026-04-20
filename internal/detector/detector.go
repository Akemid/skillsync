package detector

import (
	"os"
	"path/filepath"
)

// techIndicators maps technology names to files/dirs that indicate their presence
var techIndicators = map[string][]string{
	"go":         {"go.mod", "go.sum", "main.go"},
	"python":     {"requirements.txt", "pyproject.toml", "setup.py", "Pipfile", "poetry.lock"},
	"node":       {"package.json", "node_modules"},
	"typescript": {"tsconfig.json"},
	"rust":       {"Cargo.toml", "Cargo.lock"},
	"java":       {"pom.xml", "build.gradle", "build.gradle.kts"},
	"dotnet":     {"*.csproj", "*.sln", "*.fsproj"},
	"ruby":       {"Gemfile", "Rakefile"},
	"php":        {"composer.json"},
	"terraform":  {"*.tf", "terraform.tfstate"},
	"kubernetes": {"*.yaml", "kustomization.yaml", "Chart.yaml"},
	"docker":     {"Dockerfile", "docker-compose.yml", "docker-compose.yaml"},
	"react":      {"package.json"}, // further checked via package.json content
	"nextjs":     {"next.config.js", "next.config.mjs", "next.config.ts"},
	"fastapi":    {"requirements.txt"}, // further checked via content
	"azure":      {"azure-pipelines.yml", ".azure"},
	"aws":        {"samconfig.toml", "template.yaml", "cdk.json"},
}

// Detect returns a list of technologies detected in the given directory
func Detect(dir string) []string {
	var detected []string
	seen := make(map[string]bool)

	for tech, indicators := range techIndicators {
		for _, indicator := range indicators {
			matches, _ := filepath.Glob(filepath.Join(dir, indicator))
			if len(matches) > 0 {
				if !seen[tech] {
					detected = append(detected, tech)
					seen[tech] = true
				}
				break
			}
		}
	}

	// Special checks that need file content
	if seen["node"] || seen["python"] {
		detected = append(detected, detectFromContent(dir, seen)...)
	}

	return detected
}

// detectFromContent performs content-based detection for frameworks
func detectFromContent(dir string, seen map[string]bool) []string {
	var extra []string

	// Check package.json for React/Next.js
	if seen["node"] {
		pkgJSON := filepath.Join(dir, "package.json")
		if data, err := os.ReadFile(pkgJSON); err == nil {
			content := string(data)
			if !seen["react"] && (contains(content, `"react"`) || contains(content, `"react-dom"`)) {
				extra = append(extra, "react")
			}
			if !seen["nextjs"] && contains(content, `"next"`) {
				extra = append(extra, "nextjs")
			}
		}
	}

	// Check requirements.txt for FastAPI
	if seen["python"] {
		reqTxt := filepath.Join(dir, "requirements.txt")
		if data, err := os.ReadFile(reqTxt); err == nil {
			if !seen["fastapi"] && contains(string(data), "fastapi") {
				extra = append(extra, "fastapi")
			}
		}
	}

	return extra
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && // prevent trivial matches
		stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
