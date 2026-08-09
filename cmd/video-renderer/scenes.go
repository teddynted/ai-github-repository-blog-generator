package main

import (
	"encoding/json"
	"strings"
)

// scene is the minimal per-shot data the renderer needs: a caption (shown as a
// text card) and the narration (synthesized to speech). Duration is derived from
// the narration audio length at render time, so it is not needed here. Type is
// the storyboard scene type and Visual is the on-screen direction; both feed the
// AI scene-image prompt (see images.go).
type scene struct {
	Number    int
	Title     string
	Narration string
	Type      string
	Visual    string
}

// scenesForFormat selects the best scene source per format:
//   - youtube uses the storyboard (the long-form video plan the youtube script
//     references by scene index);
//   - youtube-shorts / tiktok use their own native vertical scripts (short,
//     fast-paced, with on-screen overlays), falling back to a capped storyboard
//     if the format script is missing/empty.
//
// scriptJSON may be nil (youtube, or when the format script was not loaded).
func scenesForFormat(format string, scriptJSON, storyboardJSON []byte) ([]scene, error) {
	switch format {
	case "youtube-shorts":
		if s := parseFormatScenes(scriptJSON, "shorts"); len(s) > 0 {
			return s, nil
		}
	case "tiktok":
		if s := parseFormatScenes(scriptJSON, "videos"); len(s) > 0 {
			return s, nil
		}
	}
	// youtube, or a fallback: the storyboard (capped for short formats).
	sb, err := parseScenes(storyboardJSON)
	if err != nil {
		return nil, err
	}
	if format == "youtube-shorts" || format == "tiktok" {
		sb = capScenes(sb, shortSceneCap)
	}
	return sb, nil
}

// unwrapArtifact unwraps the content suite's versioned envelope
// ({"promptVersion":"stage@N","artifact":{…}}) to the inner artifact bytes, so
// the renderer sees the same bare shape whether the artifact was written by the
// new versioned store or the legacy pipeline. Bare artifacts (no promptVersion)
// pass through unchanged, keeping older releases (e.g. pre-envelope content)
// renderable.
func unwrapArtifact(raw []byte) []byte {
	var env struct {
		PromptVersion string          `json:"promptVersion"`
		Artifact      json.RawMessage `json:"artifact"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.PromptVersion != "" && len(env.Artifact) > 0 {
		return env.Artifact
	}
	return raw
}

// parseScenes extracts the ordered scene list from a storyboard.json artifact
// (see internal/storyboard). Scenes with empty narration are skipped; a blank
// title falls back to "Scene".
func parseScenes(storyboardJSON []byte) ([]scene, error) {
	var sb struct {
		Scenes []rawScene `json:"scenes"`
	}
	if err := json.Unmarshal(unwrapArtifact(storyboardJSON), &sb); err != nil {
		return nil, err
	}
	return collect(sb.Scenes), nil
}

// parseFormatScenes extracts scenes from a shorts.json / tiktok.json artifact.
// Those are collections; v1 renders the first item's scenes. collectionKey is
// "shorts" or "videos". Returns nil on any parse issue (caller falls back).
func parseFormatScenes(scriptJSON []byte, collectionKey string) []scene {
	if len(scriptJSON) == 0 {
		return nil
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(unwrapArtifact(scriptJSON), &doc); err != nil {
		return nil
	}
	var items []struct {
		Scenes []rawScene `json:"scenes"`
	}
	if err := json.Unmarshal(doc[collectionKey], &items); err != nil || len(items) == 0 {
		return nil
	}
	return collect(items[0].Scenes)
}

// rawScene is the tolerant shape shared by storyboard/shorts/tiktok scenes: they
// all carry a narration and either a title (storyboard) or an overlay (short
// formats) that serves as the on-screen caption. Every format also carries an
// on-screen direction describing what to show, but under a different shape: the
// short-format scripts use a bare "visual" string; the storyboard uses a
// "visuals" object ({"description": "..."}). visualText reads whichever exists.
type rawScene struct {
	SceneNumber int             `json:"sceneNumber"`
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Overlay     string          `json:"overlay"`
	Narration   string          `json:"narration"`
	Type        string          `json:"type"`
	Visual      string          `json:"visual"`  // short-format direction (string)
	Visuals     json.RawMessage `json:"visuals"` // storyboard direction (object|string|array)
}

// visualText returns the scene's on-screen direction as plain text, from the
// short-format "visual" string or the storyboard "visuals" field (an object with
// a description, or tolerantly a bare string or array). "" when none is present.
func visualText(s rawScene) string {
	if v := strings.TrimSpace(s.Visual); v != "" {
		return v
	}
	if len(s.Visuals) == 0 {
		return ""
	}
	var obj struct {
		Description string `json:"description"`
	}
	if json.Unmarshal(s.Visuals, &obj) == nil && strings.TrimSpace(obj.Description) != "" {
		return strings.TrimSpace(obj.Description)
	}
	var str string
	if json.Unmarshal(s.Visuals, &str) == nil {
		return strings.TrimSpace(str)
	}
	var arr []string
	if json.Unmarshal(s.Visuals, &arr) == nil {
		return strings.TrimSpace(strings.Join(arr, " "))
	}
	return ""
}

func collect(raw []rawScene) []scene {
	out := make([]scene, 0, len(raw))
	for i, s := range raw {
		narr := strings.TrimSpace(s.Narration)
		if narr == "" {
			continue
		}
		n := firstNonZero(s.SceneNumber, s.Number, i+1)
		caption := firstNonEmpty(strings.TrimSpace(s.Title), strings.TrimSpace(s.Overlay), "Scene")
		out = append(out, scene{Number: n, Title: caption, Narration: narr, Type: s.Type, Visual: visualText(s)})
	}
	return out
}

// capScenes limits the scene list for short-form fallbacks. n<=0 keeps them all.
func capScenes(scenes []scene, n int) []scene {
	if n > 0 && len(scenes) > n {
		return scenes[:n]
	}
	return scenes
}

func firstNonZero(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// wrapText word-wraps s to at most cols characters per line (min 12) so a large,
// centred title fits the frame width instead of overflowing. Existing newlines
// are preserved as paragraph breaks.
func wrapText(s string, cols int) string {
	if cols < 12 {
		cols = 12
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			continue
		}
		line := words[0]
		for _, wd := range words[1:] {
			if len(line)+1+len(wd) > cols {
				out = append(out, line)
				line = wd
			} else {
				line += " " + wd
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
