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
// ANIMATED: orange particles travel the flow arrows in a short seamless loop. It
// rasterizes one SVG frame per animation phase, assembles them into a looping
// MP4, and returns its path. Returns "" when the scene names no recognizable
// component or a step fails, so the caller falls back to an AI image / title card.
//
// Exact and free: the tiles are the real services under discussion, with no
// hallucinated content and no garbled in-image text.
func diagramBackground(ctx context.Context, sc scene, work string, w, h int) string {
	text := strings.Join([]string{sc.Title, sc.Narration, sc.Visual}, " ")
	if !scenediagram.Has(text) {
		return ""
	}
	dir := filepath.Join(work, fmt.Sprintf("diag_%d", sc.Number))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	n := scenediagram.AnimationFrames
	for i := 0; i < n; i++ {
		svg := scenediagram.SVG(text, w, h, float64(i)/float64(n))
		if svg == "" {
			return ""
		}
		svgPath := filepath.Join(dir, fmt.Sprintf("f_%03d.svg", i))
		if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil { // #nosec G703 -- dir is an os.MkdirTemp subdir; filename is constant
			return ""
		}
		pngPath := filepath.Join(dir, fmt.Sprintf("f_%03d.png", i))
		cmd := exec.CommandContext(ctx, rsvgBin, "-w", itoa(w), "-h", itoa(h), "-o", pngPath, svgPath)
		if err := cmd.Run(); err != nil {
			log.Printf("scene %d diagram frame %d failed, falling back: %v", sc.Number, i, err)
			return ""
		}
	}
	// Assemble a seamless loop: n frames at n fps (a ~1s cycle), re-timed to 30fps.
	loop := filepath.Join(work, fmt.Sprintf("scene_%d_diagram.mp4", sc.Number))
	cmd := exec.CommandContext(ctx, ffmpegBin, "-y",
		"-framerate", itoa(n), "-i", filepath.Join(dir, "f_%03d.png"),
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-r", "30", loop)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("scene %d diagram loop assemble failed, falling back: %v", sc.Number, err)
		return ""
	}
	log.Printf("scene %d: composed animated architecture diagram [%s]", sc.Number,
		strings.Join(scenediagram.Detect(text, 4), " → "))
	return loop
}
