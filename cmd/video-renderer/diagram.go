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
// rasterizes it to a PNG, and returns its path. Returns "" when the scene names
// no recognizable component or rasterization fails, so the caller falls back to
// an AI image or the title card.
//
// Unlike the AI backend this is exact and free: the tiles are the real services
// under discussion, with no hallucinated content and no garbled in-image text.
func diagramBackground(ctx context.Context, sc scene, work string, w, h int) string {
	text := strings.Join([]string{sc.Title, sc.Narration, sc.Visual}, " ")
	svg := scenediagram.SVG(text, w, h)
	if svg == "" {
		return ""
	}
	svgPath := filepath.Join(work, fmt.Sprintf("scene_%d_diagram.svg", sc.Number))
	if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil { // #nosec G703 -- work is an os.MkdirTemp dir; filename is constant
		return ""
	}
	png := filepath.Join(work, fmt.Sprintf("scene_%d_diagram.png", sc.Number))
	cmd := exec.CommandContext(ctx, rsvgBin, "-w", itoa(w), "-h", itoa(h), "-o", png, svgPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("scene %d diagram rasterize failed, falling back: %v", sc.Number, err)
		return ""
	}
	log.Printf("scene %d: composed architecture diagram [%s]", sc.Number,
		strings.Join(scenediagram.Detect(text, 4), " → "))
	return png
}
