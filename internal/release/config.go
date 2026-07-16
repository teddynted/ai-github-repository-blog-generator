// Package release orchestrates the semantic-versioning release workflow:
// determine the next version from Conventional Commits, generate release notes
// and a CHANGELOG section, create and push an annotated Git tag, and publish a
// GitHub Release. It depends on small ports (Git, GitHubReleases) so the
// workflow is deterministic and unit-testable.
package release

import (
	"encoding/json"
	"os"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/changelog"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/conventional"
)

// Config is the release configuration. All fields have sensible defaults via
// Default(); a project may override them in a JSON file (see Load).
type Config struct {
	// InitialVersion is used when no tags exist yet (e.g. "0.1.0").
	InitialVersion string `json:"initial_version"`
	// TagPrefix is prepended to versions to form tag names (e.g. "v" -> v1.2.3).
	TagPrefix string `json:"tag_prefix"`
	// ReleaseBranch is the only branch a release may be cut from (e.g. "main").
	ReleaseBranch string `json:"release_branch"`
	// PreReleaseID is the default pre-release identifier for `--pre` (e.g. "rc").
	PreReleaseID string `json:"prerelease_id"`
	// ChangelogCategories overrides the default changelog grouping.
	ChangelogCategories []changelog.Category `json:"changelog_categories,omitempty"`
	// Commit controls Conventional Commit parsing + bump derivation.
	Commit conventional.Config `json:"commit,omitempty"`
	// IgnoredTypes are commit types that never, on their own, warrant a release
	// (informational; bump derivation already ignores non-minor/patch types).
	IgnoredTypes []string `json:"ignored_types,omitempty"`
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		InitialVersion:      "0.1.0",
		TagPrefix:           "v",
		ReleaseBranch:       "main",
		PreReleaseID:        "rc",
		ChangelogCategories: changelog.DefaultCategories,
		Commit:              conventional.Config{},
		IgnoredTypes:        []string{"docs", "style", "test", "chore", "ci", "build"},
	}
}

// Load reads config overrides from a JSON file, falling back to Default() for
// any unset field. A missing file is not an error — defaults are returned.
func Load(path string) (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Default(), err
	}
	// Re-apply defaults for any field the file left empty.
	d := Default()
	if cfg.InitialVersion == "" {
		cfg.InitialVersion = d.InitialVersion
	}
	if cfg.TagPrefix == "" {
		cfg.TagPrefix = d.TagPrefix
	}
	if cfg.ReleaseBranch == "" {
		cfg.ReleaseBranch = d.ReleaseBranch
	}
	if len(cfg.ChangelogCategories) == 0 {
		cfg.ChangelogCategories = d.ChangelogCategories
	}
	return cfg, nil
}
