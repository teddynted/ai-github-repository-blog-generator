package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type storeDoc struct{ N int }

func TestArtifactStoreRoundtrip(t *testing.T) {
	dir := t.TempDir()
	blog := filepath.Join(dir, "blog.md")
	if err := os.WriteFile(blog, []byte("blog"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := newArtifactStore(dir, blog)

	if err := st.Save("storyboard", storeDoc{N: 7}, "storyboard@1"); err != nil {
		t.Fatalf("save: %v", err)
	}
	var d storeDoc
	ver, ok, err := st.Load("storyboard", &d)
	if err != nil || !ok || d.N != 7 || ver != "storyboard@1" {
		t.Fatalf("roundtrip: ver=%q ok=%v err=%v d=%+v", ver, ok, err, d)
	}
	if r, s := st.Stats(); r != 1 || s != 1 {
		t.Errorf("stats: reused=%d saved=%d", r, s)
	}

	// A missing sidecar is simply not found (regenerate), not an error.
	if _, ok, err := st.Load("missing", &d); ok || err != nil {
		t.Errorf("missing: ok=%v err=%v", ok, err)
	}
}

func TestArtifactStoreStaleWhenOlderThanBlog(t *testing.T) {
	dir := t.TempDir()
	blog := filepath.Join(dir, "blog.md")
	if err := os.WriteFile(blog, []byte("blog"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := newArtifactStore(dir, blog)
	if err := st.Save("voiceover", storeDoc{N: 1}, "voiceover@1"); err != nil {
		t.Fatal(err)
	}
	// Make the blog newer than the sidecar → the sidecar is stale.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(blog, future, future); err != nil {
		t.Fatal(err)
	}
	st2 := newArtifactStore(dir, blog)
	var d storeDoc
	if _, ok, _ := st2.Load("voiceover", &d); ok {
		t.Error("a sidecar older than the blog must be treated as stale (regenerate)")
	}
}
