package releasecontext

import (
	"fmt"
	"sort"
	"strings"
)

// dirResponsibility maps a well-known top-level directory to its role. Unknown
// directories still appear, with a generic responsibility.
var dirResponsibility = map[string]string{
	"cmd":            "Executable entry points (main packages)",
	"internal":       "Private application and business logic packages",
	"pkg":            "Public, reusable library packages",
	"lambdas":        "AWS Lambda function entry points",
	"docs":           "Project documentation",
	"diagrams":       "Architecture and workflow diagrams",
	"blog":           "Generated or authored blog content",
	"scripts":        "Automation and operational scripts",
	".github":        "GitHub configuration and CI/CD workflows",
	"test":           "Test suites and fixtures",
	"tests":          "Test suites and fixtures",
	"deployments":    "Deployment manifests and environment config",
	"deployment":     "Deployment manifests and environment config",
	"infrastructure": "Infrastructure as Code (CloudFormation templates)",
	"instance":       "Instance provisioning and runtime configuration",
	"packer":         "Machine image (AMI) build definitions",
	"api":            "API definitions and handlers",
	"web":            "Web/front-end assets",
	"config":         "Configuration files",
}

// analyzeStructure classifies top-level directories and derives an overview.
func analyzeStructure(files []RawFile) RepositoryStructure {
	counts := map[string]int{}
	total := 0
	for _, f := range files {
		total++
		top := topDir(f.Path)
		if top == "" {
			top = "(root)"
		}
		counts[top]++
	}

	dirs := make([]DirectoryInfo, 0, len(counts))
	for name, n := range counts {
		if name == "(root)" {
			continue
		}
		resp, ok := dirResponsibility[name]
		if !ok {
			resp = "Project directory"
		}
		dirs = append(dirs, DirectoryInfo{Path: name + "/", Responsibility: resp, FileCount: n})
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Path < dirs[j].Path })

	rs := RepositoryStructure{Directories: dirs, FileCount: total, Layout: detectLayout(counts)}
	rs.Overview = structureOverview(rs)
	return rs
}

// topDir returns the first path segment, or "" for a root-level file.
func topDir(p string) string {
	p = strings.TrimPrefix(p, "./")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

// detectLayout names the recognizable project layout.
func detectLayout(counts map[string]int) string {
	_, hasCmd := counts["cmd"]
	_, hasInternal := counts["internal"]
	_, hasLambdas := counts["lambdas"]
	_, hasInfra := counts["infrastructure"]
	switch {
	case hasCmd && hasInternal && hasLambdas && hasInfra:
		return "standard-go-serverless-monorepo"
	case hasCmd && hasInternal:
		return "standard-go"
	case hasLambdas:
		return "serverless"
	default:
		return "custom"
	}
}

func structureOverview(rs RepositoryStructure) string {
	if len(rs.Directories) == 0 {
		return "A flat repository with no top-level package directories."
	}
	names := make([]string, 0, len(rs.Directories))
	for _, d := range rs.Directories {
		names = append(names, strings.TrimSuffix(d.Path, "/"))
	}
	return fmt.Sprintf(
		"A %s project of %d files organized into %d top-level directories (%s), separating entry points, business logic, infrastructure, and documentation.",
		rs.Layout, rs.FileCount, len(rs.Directories), strings.Join(names, ", "),
	)
}
