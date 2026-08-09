package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/scenediagram"
)

// diagramBackground builds a deterministic, on-brand architecture diagram for a
// scene from the services it names (composited service-icon tiles + flow arrows),
// ANIMATED: orange particles travel the flow arrows in a short seamless loop.
// Returns "" when the scene names no recognizable component (caller falls back to
// a card). Exact and free: the tiles are the real services under discussion, with
// no hallucinated content and no garbled in-image text.
func diagramBackground(ctx context.Context, sc scene, work string, w, h int) string {
	text := strings.Join([]string{sc.Title, sc.Narration, sc.Visual}, " ")
	// A diagram needs a RELATIONSHIP to show — at least two connected components. A
	// scene that names a single component (common when a short-format narration is
	// trimmed) is expanded with that component's canonical neighbors so it still
	// forms a meaningful flow (e.g. OpenClaw → Ollama → Bedrock) rather than a lone
	// tile; only a lone component with no neighbors falls back to the heading card.
	keys := scenediagram.Detect(text, 4)
	if len(keys) < 2 {
		keys = scenediagram.WithNeighbors(keys)
	}
	if len(keys) < 2 {
		return ""
	}
	return animatedDiagram(ctx, fmt.Sprintf("scene_%d", sc.Number), work, w, h,
		func(phase float64) string { return scenediagram.Compose(keys, w, h, phase) }, keys)
}

// overviewMaxNodes caps the opening overview so the hero spine stays legible on a
// vertical phone (the full component set is too dense).
const overviewMaxNodes = 6

// overviewKeys collects every component named across all scenes (the release's
// architecture, since scenes derive from it) and returns the flow-ordered hero
// spine, capped — source → ingress → compute → workflow → orchestrator →
// inference. Empty when the release names nothing diagrammable.
func overviewKeys(scenes []scene) []string {
	// Skip low-level or redundant nodes in the high-level overview: "webhook" is how
	// GitHub delivers (the source is already shown), and "claude" is a redundant
	// third inference option alongside Ollama/Bedrock. They still appear in the
	// detailed per-scene diagrams.
	skip := map[string]bool{"webhook": true, "claude": true}
	seen := map[string]bool{}
	var all []string
	for _, sc := range scenes {
		text := strings.Join([]string{sc.Title, sc.Narration, sc.Visual}, " ")
		for _, k := range scenediagram.Detect(text, 0) {
			if !seen[k] && !skip[k] {
				seen[k] = true
				all = append(all, k)
			}
		}
	}
	sp := scenediagram.FlowSort(all)
	if len(sp) > overviewMaxNodes {
		sp = sp[:overviewMaxNodes]
	}
	return sp
}

// overviewBackground builds the opening SYSTEM-OVERVIEW diagram from an explicit,
// flow-ordered set of the release's components (the curated hero spine, e.g.
// GitHub → EventBridge → Lambda → n8n → OpenClaw → Ollama), animated the same way.
// Returns "" for an empty set or on failure (caller falls back to the title card).
func overviewBackground(ctx context.Context, keys []string, work string, w, h int) string {
	if len(keys) < 2 { // need a flow to show; a lone tile falls back to the title card
		return ""
	}
	return animatedDiagram(ctx, "overview", work, w, h,
		func(phase float64) string { return scenediagram.Compose(keys, w, h, phase) }, keys)
}

// animatedDiagram rasterizes one SVG frame per animation phase (via svgFn) and
// assembles them into a seamless looping MP4, returning its path. labels are only
// for the log line. Returns "" on any failure so the caller can fall back.
func animatedDiagram(ctx context.Context, id, work string, w, h int, svgFn func(phase float64) string, labels []string) string {
	dir := filepath.Join(work, "diag_"+id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	n := scenediagram.AnimationFrames
	for i := 0; i < n; i++ {
		svg := svgFn(float64(i) / float64(n))
		if svg == "" {
			return ""
		}
		svgPath := filepath.Join(dir, fmt.Sprintf("f_%03d.svg", i))
		if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil { // #nosec G703 -- dir is an os.MkdirTemp subdir; filename is constant
			return ""
		}
		pngPath := filepath.Join(dir, fmt.Sprintf("f_%03d.png", i))
		if err := exec.CommandContext(ctx, rsvgBin, "-w", itoa(w), "-h", itoa(h), "-o", pngPath, svgPath).Run(); err != nil {
			log.Printf("diagram %s frame %d failed, falling back: %v", id, i, err)
			return ""
		}
	}
	// Assemble a seamless loop: n frames at n fps (a ~1s cycle), re-timed to 30fps.
	loop := filepath.Join(work, "diag_"+id+".mp4")
	cmd := exec.CommandContext(ctx, ffmpegBin, "-y",
		"-framerate", itoa(n), "-i", filepath.Join(dir, "f_%03d.png"),
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-r", "30", loop)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("diagram %s loop assemble failed, falling back: %v", id, err)
		return ""
	}
	log.Printf("composed animated architecture diagram %s [%s]", id, strings.Join(labels, " → "))
	return loop
}
