package reposource

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
)

// FSAnalyzer detects a repository's technical profile from its working copy:
// languages, dependency managers, IaC, containers, and CI/CD. Detection is
// filename/extension based (fast), with a bounded content check to tell
// CloudFormation apart from other YAML.
type FSAnalyzer struct{}

var ignoredDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, "target": true, ".idea": true, ".vscode": true, ".terraform": true,
}

var extLang = map[string]string{
	".go": "Go", ".js": "JavaScript", ".jsx": "JavaScript", ".ts": "TypeScript",
	".tsx": "TypeScript", ".py": "Python", ".rb": "Ruby", ".rs": "Rust",
	".java": "Java", ".kt": "Kotlin", ".php": "PHP", ".cs": "C#", ".c": "C",
	".cpp": "C++", ".cc": "C++", ".swift": "Swift", ".scala": "Scala", ".ex": "Elixir",
}

const maxCFNChecks = 60

// Analyze walks the working copy and returns the detected profile.
func (FSAnalyzer) Analyze(_ context.Context, localPath string) (processing.Analysis, error) {
	langs, pms, iac, containers, cicd := newSet(), newSet(), newSet(), newSet(), newSet()
	cfnChecks := 0

	err := filepath.WalkDir(localPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := strings.ToLower(d.Name())
		if d.IsDir() {
			if path != localPath && ignoredDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		rel := filepath.ToSlash(strings.TrimPrefix(path, localPath))

		if l, ok := extLang[ext]; ok {
			langs.add(l)
		}
		switch ext {
		case ".tf", ".tfvars":
			iac.add("Terraform")
		}
		switch name {
		case "go.mod":
			pms.add("Go modules")
		case "package.json":
			pms.add("npm/Node")
		case "yarn.lock":
			pms.add("Yarn")
		case "pnpm-lock.yaml":
			pms.add("pnpm")
		case "requirements.txt", "setup.py":
			pms.add("pip")
		case "pyproject.toml":
			pms.add("Python (pyproject)")
		case "pipfile":
			pms.add("pipenv")
		case "pom.xml":
			pms.add("Maven")
		case "build.gradle", "build.gradle.kts":
			pms.add("Gradle")
		case "cargo.toml":
			pms.add("Cargo")
		case "gemfile":
			pms.add("Bundler")
		case "composer.json":
			pms.add("Composer")
		case "dockerfile":
			containers.add("Docker")
		case "docker-compose.yml", "docker-compose.yaml", "compose.yaml", "compose.yml":
			containers.add("Docker Compose")
		case ".gitlab-ci.yml":
			cicd.add("GitLab CI")
		case "jenkinsfile":
			cicd.add("Jenkins")
		}
		if strings.HasSuffix(strings.ToLower(name), ".dockerfile") {
			containers.add("Docker")
		}
		if strings.Contains(rel, "/.github/workflows/") && (ext == ".yml" || ext == ".yaml") {
			cicd.add("GitHub Actions")
		}
		if strings.Contains(rel, "/.circleci/") {
			cicd.add("CircleCI")
		}
		if (ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".template") && cfnChecks < maxCFNChecks {
			cfnChecks++
			if looksLikeCloudFormation(path) {
				iac.add("CloudFormation")
			}
		}
		return nil
	})
	if err != nil {
		return processing.Analysis{}, err
	}
	return processing.Analysis{
		Languages:       langs.sorted(),
		PackageManagers: pms.sorted(),
		IaC:             iac.sorted(),
		Containers:      containers.sorted(),
		CICD:            cicd.sorted(),
	}, nil
}

// looksLikeCloudFormation reads a bounded prefix and checks for CFN markers.
func looksLikeCloudFormation(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 16*1024)
	n, _ := f.Read(buf)
	head := string(buf[:n])
	return strings.Contains(head, "AWSTemplateFormatVersion") || strings.Contains(head, "Type: AWS::")
}

type stringset map[string]struct{}

func newSet() stringset { return stringset{} }

func (s stringset) add(v string) { s[v] = struct{}{} }

func (s stringset) sorted() []string {
	if len(s) == 0 {
		return nil
	}
	out := make([]string, 0, len(s))
	for v := range s {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
