package main

import (
	"encoding/json"
	"strings"
)

// scene is the minimal per-shot data the renderer needs: a caption (shown as a
// text card) and the narration (synthesized to speech). Duration is derived from
// the narration audio length at render time, so it is not needed here.
type scene struct {
	Number    int
	Title     string
	Narration string
}

// parseScenes extracts the ordered scene list from a storyboard.json artifact
// (see internal/storyboard). It is tolerant: scenes with empty narration are
// skipped, and a blank title falls back to "Scene N".
func parseScenes(storyboardJSON []byte) ([]scene, error) {
	var sb struct {
		Scenes []struct {
			SceneNumber int    `json:"sceneNumber"`
			Title       string `json:"title"`
			Narration   string `json:"narration"`
		} `json:"scenes"`
	}
	if err := json.Unmarshal(storyboardJSON, &sb); err != nil {
		return nil, err
	}
	out := make([]scene, 0, len(sb.Scenes))
	for i, s := range sb.Scenes {
		narr := strings.TrimSpace(s.Narration)
		if narr == "" {
			continue
		}
		n := s.SceneNumber
		if n == 0 {
			n = i + 1
		}
		title := strings.TrimSpace(s.Title)
		if title == "" {
			title = "Scene"
		}
		out = append(out, scene{Number: n, Title: title, Narration: narr})
	}
	return out, nil
}

// cap limits the scene list for short-form formats (youtube-shorts, tiktok) so a
// vertical clip does not run the full long-form length. n<=0 keeps them all.
func capScenes(scenes []scene, n int) []scene {
	if n > 0 && len(scenes) > n {
		return scenes[:n]
	}
	return scenes
}
