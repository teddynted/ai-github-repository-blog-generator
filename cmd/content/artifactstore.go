package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// artifactStore persists each stage's STRUCTURED output as a JSON sidecar under
// <releaseDir>/.artifacts/, so a later targeted run reloads a dependency instead
// of regenerating it (an expensive model call). Freshness is exists-based: a
// sidecar is reusable when it exists and is at least as new as the source blog —
// so editing (or regenerating) the blog invalidates the whole downstream chain,
// while iterating on a single artifact reuses everything else.
type artifactStore struct {
	dir     string    // <releaseDir>/.artifacts
	blogRef time.Time // source blog modtime; sidecars older than this are stale (zero ⇒ exists-only)
	hits    int64
	saves   int64
}

// newArtifactStore builds a store rooted at releaseDir. blogPath, when non-empty
// and readable, sets the freshness reference to the blog's modtime.
func newArtifactStore(releaseDir, blogPath string) *artifactStore {
	s := &artifactStore{dir: filepath.Join(releaseDir, ".artifacts")}
	if blogPath != "" {
		if fi, err := os.Stat(blogPath); err == nil {
			s.blogRef = fi.ModTime()
		}
	}
	return s
}

func (s *artifactStore) path(stage string) string {
	return filepath.Join(s.dir, stage+".json")
}

// Load decodes the stage's sidecar into v when it exists and is fresh. A missing
// or stale sidecar returns found=false (the stage regenerates); a corrupt sidecar
// returns an error so the caller can log and regenerate.
func (s *artifactStore) Load(stage string, v any) (bool, error) {
	p := s.path(stage)
	fi, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	// Stale when older than the source blog — the blog changed, so downstream
	// artifacts derived from it must be regenerated.
	if !s.blogRef.IsZero() && fi.ModTime().Before(s.blogRef) {
		return false, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return false, err
	}
	atomic.AddInt64(&s.hits, 1)
	return true, nil
}

// Save writes the stage's struct as a pretty JSON sidecar.
func (s *artifactStore) Save(stage string, v any) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path(stage), b, 0o644); err != nil {
		return err
	}
	atomic.AddInt64(&s.saves, 1)
	return nil
}

// Stats returns how many artifacts were reused and how many were (re)persisted.
func (s *artifactStore) Stats() (reused, saved int) {
	return int(atomic.LoadInt64(&s.hits)), int(atomic.LoadInt64(&s.saves))
}
