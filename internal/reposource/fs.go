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

const (
	maxFileBytes = 256 * 1024 // cap per file read into a snapshot
	maxDocs      = 20         // cap number of documentation files
)

// FSReadme reads the repository README from a local working copy.
type FSReadme struct{}

// Readme returns the README contents, or "" (no error) if there is none.
func (FSReadme) Readme(_ context.Context, localPath string) (string, error) {
	entries, err := os.ReadDir(localPath)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if lower == "readme.md" || lower == "readme" || lower == "readme.markdown" {
			return readCapped(filepath.Join(localPath, e.Name()))
		}
	}
	return "", nil
}

// FSDocs reads Markdown documentation from a local working copy.
type FSDocs struct{}

// Docs returns Markdown files under docs/ (recursively), capped in count and
// size, with repo-relative paths. Missing docs/ yields an empty result.
func (FSDocs) Docs(_ context.Context, localPath string) ([]processing.Document, error) {
	docsDir := filepath.Join(localPath, "docs")
	info, err := os.Stat(docsDir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	var docs []processing.Document
	err = filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		if len(docs) >= maxDocs {
			return filepath.SkipAll
		}
		content, rerr := readCapped(path)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(localPath, path)
		docs = append(docs, processing.Document{Path: filepath.ToSlash(rel), Content: content})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs, nil
}

// readCapped reads at most maxFileBytes from a file.
func readCapped(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, maxFileBytes)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		// Empty file reads as io.EOF with n==0; treat as empty content.
		return "", nil
	}
	return string(buf[:n]), nil
}
