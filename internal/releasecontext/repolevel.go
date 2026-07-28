package releasecontext

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// repoLevelIgnoredDirs are directories excluded from the working-tree inventory:
// VCS internals, dependency/build output, editor state, and this tool's own
// generated artifacts and caches (which are not part of the architecture).
var repoLevelIgnoredDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, "target": true, ".idea": true, ".vscode": true, ".terraform": true,
	"output": true, ".cache": true, ".claude": true, "tmp": true, "fixtures": true,
	"testdata": true, "logs": true,
}

// repoLevelTextExt are the extensions whose content is read into the inventory.
// Non-text files still contribute their path (for structure) but not content.
var repoLevelTextExt = map[string]bool{
	".md": true, ".go": true, ".yaml": true, ".yml": true, ".json": true,
	".toml": true, ".mod": true, ".sum": true, ".txt": true, ".sh": true,
	".tf": true, ".tfvars": true, ".mmd": true,
}

const (
	repoLevelMaxFileBytes  = 256 * 1024       // per-file content cap
	repoLevelMaxTotalBytes = 12 * 1024 * 1024 // total inventory content cap
)

// BuildRepoLevel assembles a version-independent Release Context from a local
// working tree (no GitHub, no release). It reads the repository's files into an
// inventory and reuses the same analyzers as the release-based Builder — so the
// documentation, structure, CloudFormation, technologies, Mermaid, and
// architecture analyses are identical — but leaves the Release empty. repoName is
// the repository name for the Repository metadata; when empty, the base name of
// localPath is used. It is the input for repository-level, version-agnostic
// artifacts such as docs/architecture.md.
func BuildRepoLevel(ctx context.Context, localPath, repoName string) (*ReleaseContext, error) {
	files, err := inventory(localPath)
	if err != nil {
		return nil, err
	}
	if repoName == "" {
		if abs, aerr := filepath.Abs(localPath); aerr == nil {
			repoName = filepath.Base(abs)
		} else {
			repoName = filepath.Base(localPath)
		}
	}

	rc := &ReleaseContext{
		SchemaVersion: SchemaVersion,
		ContextID:     newContextID(),
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	rc.Repository = buildRepository(RawRepository{Name: repoName, FullName: repoName})
	// Release intentionally left zero — this context is version-independent.
	rc.Documentation = analyzeDocumentation(files)
	rc.RepositoryStructure = analyzeStructure(files)
	rc.CloudFormation = analyzeCloudFormation(files)
	rc.Technologies = analyzeTechnologies(files, rc.CloudFormation.Services)
	rc.Mermaid = analyzeMermaid(files)

	// Depend on the analyses above.
	rc.Architecture = analyzeArchitecture(rc)
	rc.Implementation = buildImplementation(rc)
	rc.ContentIntelligence = buildContentIntelligence(rc)
	return rc, nil
}

// inventory walks the working tree into a []RawFile: every non-ignored file
// contributes its repo-relative path (so structure analysis sees the layout),
// and text files additionally contribute their (capped) content.
func inventory(root string) ([]RawFile, error) {
	var files []RawFile
	total := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			// Skip ignored dirs and Go-convention "_"-prefixed dirs (build/temp),
			// but keep dot-dirs like .github/.githooks (CI/CD signal).
			if p != root && (repoLevelIgnoredDirs[strings.ToLower(name)] || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(p, root), "/"))
		if rel == "" {
			return nil
		}
		info, ierr := d.Info()
		var size int64
		if ierr == nil {
			size = info.Size()
		}
		rf := RawFile{Path: rel, Size: size}
		if repoLevelTextExt[strings.ToLower(filepath.Ext(rel))] && total < repoLevelMaxTotalBytes {
			if content := readCappedFile(p); content != "" {
				rf.Content = content
				total += len(content)
			}
		}
		files = append(files, rf)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// readCappedFile reads up to repoLevelMaxFileBytes of a file, ignoring errors
// (an unreadable file simply contributes no content).
func readCappedFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, repoLevelMaxFileBytes)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return ""
	}
	return string(buf[:n])
}
