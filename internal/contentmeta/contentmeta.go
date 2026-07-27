// Package contentmeta builds the per-release metadata.json that accompanies a
// batch of published artifacts. It records only the provenance that Amazon S3
// does not already store (size, ETag, Content-Type, Last-Modified, storage
// class, version id are all in S3 object metadata) — the semantic "how was this
// generated" that makes runs reproducible and comparable: generation id, git
// commit, timestamp, generator version, and per-artifact provider, model,
// prompt version, content hash, and S3 object version.
package contentmeta

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// SchemaVersion is the metadata.json schema version (SemVer, additive-only).
const SchemaVersion = "1.0.0"

// Kind/Ext of the metadata artifact itself.
const (
	Kind = "metadata"
	Ext  = "json"
)

// ArtifactMeta is the provenance of one published artifact.
type ArtifactMeta struct {
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	PromptVersion string `json:"promptVersion,omitempty"`
	S3VersionID   string `json:"s3VersionId,omitempty"`
	SHA256        string `json:"sha256"`
}

// Metadata is the release generation manifest written as metadata.json.
type Metadata struct {
	SchemaVersion    string                  `json:"schemaVersion"`
	GenerationID     string                  `json:"generationId"`
	Owner            string                  `json:"owner"`
	Repository       string                  `json:"repository"`
	ReleaseTag       string                  `json:"releaseTag"`
	GitCommit        string                  `json:"gitCommit,omitempty"`
	GeneratedAt      string                  `json:"generatedAt"`
	GeneratorVersion string                  `json:"generatorVersion,omitempty"`
	Artifacts        map[string]ArtifactMeta `json:"artifacts"`
}

// Options carries run-level facts the builder cannot derive from the assets.
type Options struct {
	GeneratorVersion string
	// Now and NewID are injectable for deterministic tests; nil uses the defaults.
	Now   func() time.Time
	NewID func() string
}

// Build assembles the metadata for a batch of published assets. results (from a
// versioned publisher) supplies each artifact's S3 object version; when absent
// (e.g. filesystem publishing) the version is simply omitted. The metadata
// artifact itself is never included in the manifest.
func Build(rctx *rc.ReleaseContext, assets []generation.Content, results []generation.PutResult, opts Options) Metadata {
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	newID := NewGenerationID
	if opts.NewID != nil {
		newID = opts.NewID
	}

	versionByKey := make(map[string]string, len(results))
	for _, r := range results {
		versionByKey[r.Kind+"."+extOr(r.Ext)] = r.VersionID
	}

	arts := make(map[string]ArtifactMeta, len(assets))
	for _, a := range assets {
		if string(a.Kind) == Kind {
			continue // never describe the manifest in itself
		}
		filename := string(a.Kind) + "." + extOr(a.Ext)
		sum := sha256.Sum256([]byte(a.Markdown))
		arts[filename] = ArtifactMeta{
			Provider:      a.Provider,
			Model:         a.Model,
			PromptVersion: a.PromptVersion,
			S3VersionID:   versionByKey[filename],
			SHA256:        hex.EncodeToString(sum[:]),
		}
	}

	return Metadata{
		SchemaVersion:    SchemaVersion,
		GenerationID:     newID(),
		Owner:            rctx.Repository.Owner,
		Repository:       rctx.Repository.FullName,
		ReleaseTag:       rctx.Release.Tag,
		GitCommit:        releaseCommit(rctx),
		GeneratedAt:      now().UTC().Format(time.RFC3339),
		GeneratorVersion: opts.GeneratorVersion,
		Artifacts:        arts,
	}
}

// Content renders the metadata as a publishable generation.Content (metadata.json)
// carrying the same release tag so it lands beside the artifacts it describes.
func (m Metadata) Content() (generation.Content, error) {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return generation.Content{}, err
	}
	return generation.Content{Kind: Kind, Markdown: string(b) + "\n", Release: m.ReleaseTag, Ext: Ext}, nil
}

// releaseCommit returns the newest analysed commit SHA as a best-effort release
// head (the commit range's head), or "" when none is known.
func releaseCommit(rctx *rc.ReleaseContext) string {
	if len(rctx.Commits) > 0 {
		return rctx.Commits[0].SHA
	}
	return ""
}

func extOr(ext string) string {
	if ext == "" {
		return "md"
	}
	return ext
}

// NewGenerationID returns a sortable, unique generation id: a millisecond
// timestamp prefix (hex) plus random entropy, so ids sort chronologically and
// never collide, without adding a UUID/ULID dependency.
func NewGenerationID() string {
	ts := uint64(time.Now().UnixMilli())
	var r [8]byte
	_, _ = rand.Read(r[:])
	return fmt.Sprintf("%012x%s", ts, hex.EncodeToString(r[:]))
}
