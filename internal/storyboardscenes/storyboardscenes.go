// Package storyboardscenes converts a generated Storyboard (M4) into a canonical
// per-scene SDXL visual specification — the storyboard-scenes artifact. Each
// scene becomes one independently renderable image spec (a Replicate
// stability-ai/sdxl prompt + negative prompt + camera + motion + duration), so a
// downstream Fargate worker can render a picture per scene and an FFmpeg pipeline
// can assemble them into short- and long-form video.
//
// It is a DETERMINISTIC transform: every field is derived from the storyboard's
// own scenes (type, objective, narration, visuals, camera, animations, timing).
// It invents no architecture and needs no model call.
package storyboardscenes

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
)

// SchemaVersion is the storyboard-scenes document version (SemVer, additive-only).
const SchemaVersion = "1.0.0"

// ModelTarget is the image model these scene prompts are tuned for.
const ModelTarget = "stability-ai/sdxl"

// SharedStylePrompt is the reusable style anchor prepended to every scene prompt.
const SharedStylePrompt = "Flat-vector technical illustration, subtle isometric depth, dark navy gradient background, faint grid texture, AWS-inspired engineering aesthetic, soft directional lighting, gentle rim light, crisp edges, modern, confident, engineering-credible, generous negative space. Depict RECOGNIZABLE, stylized cloud-infrastructure iconography that evokes the subject — service nodes, event buses, message queues, serverless functions, compute instances, storage volumes, and the directional arrows and data/event flows connecting them. Completely WORDLESS: no text, no words, no letters, no numbers, no labels, no captions, no writing, no UI, no dashboards, no logos, no watermarks of any kind — convey meaning through iconography and flow, never through text."

// SharedQualitySuffix is the mandatory cinematic quality modifier appended to
// every SDXL prompt, so scene imagery reads as art-directed, enterprise-grade
// visuals rather than generic renders. Coherent with the flat-vector brand look.
const SharedQualitySuffix = "cinematic lighting, volumetric depth, soft global illumination, crisp clean vector edges, ultra-detailed, professional enterprise technical illustration, high production value, art-directed color grading, sharp focus, 4k, award-winning design"

// SharedNegativePrompt is the reusable negative prompt applied to every scene.
const SharedNegativePrompt = "text, words, letters, numbers, labels, captions, writing, typography, lettering, alphanumeric characters, gibberish text, fake text, UI text, screen text, dashboards, terminal text, code, source code, logos, watermarks, signatures, blurry, blurry details, clutter, cluttered, low detail, excessive visual noise, photorealistic humans, distorted infrastructure, tangled connectors, low resolution, unreadable shapes"

// compositions are the cinematic layouts scenes rotate through so consecutive
// scenes are visually distinct.
var compositions = []string{
	"centered orchestration hub",
	"right-weighted reveal",
	"diagonal event cascade",
	"layered infrastructure stack",
	"radial event burst",
	"layered observability planes",
	"immutable infrastructure pipeline",
}

// cameraMoves are the FFmpeg-friendly camera directions scenes rotate through
// when the storyboard scene does not name one.
var cameraMoves = []string{
	"slow push in", "slow pull out", "left pan", "right pan",
	"parallax drift", "orbit move", "diagonal reveal", "zoom and hold",
}

// SceneSpec is one independently renderable scene.
type SceneSpec struct {
	Number             int    `json:"number"`
	Purpose            string `json:"purpose"`
	NarrationAlignment string `json:"narrationAlignment"`
	CameraDirection    string `json:"cameraDirection"`
	Composition        string `json:"composition"`
	SDXLPrompt         string `json:"sdxlPrompt"`
	NegativePrompt     string `json:"negativePrompt"`
	MotionSuggestion   string `json:"motionSuggestion"`
	DurationSec        int    `json:"durationSec"`
}

// SceneCollection is the full storyboard-scenes artifact for one release.
type SceneCollection struct {
	SchemaVersion string      `json:"schemaVersion"`
	Repository    string      `json:"repository"`
	Release       string      `json:"release"`
	SourceTitle   string      `json:"sourceTitle"`
	ModelTarget   string      `json:"modelTarget"`
	Scenes        []SceneSpec `json:"scenes"`
}

// Build converts a storyboard into a per-scene SDXL specification. Scenes with
// empty narration are skipped (nothing to align a visual to).
func Build(sb storyboard.Storyboard, repository, release string) SceneCollection {
	col := SceneCollection{
		SchemaVersion: SchemaVersion,
		Repository:    repository,
		Release:       release,
		SourceTitle:   sb.Metadata.SourceBlogTitle,
		ModelTarget:   ModelTarget,
	}
	base := hashBase(release)
	n := 0
	for _, sc := range sb.Scenes {
		if strings.TrimSpace(sc.Narration) == "" {
			continue
		}
		n++
		comp := compositions[(base+n)%len(compositions)]
		col.Scenes = append(col.Scenes, SceneSpec{
			Number:             n,
			Purpose:            purposeFor(sc.Type),
			NarrationAlignment: narrationLead(firstNonEmpty(sc.Narration, sc.Objective)),
			CameraDirection:    cameraFor(sc, base+n),
			Composition:        comp,
			SDXLPrompt:         buildPrompt(sc, comp),
			NegativePrompt:     SharedNegativePrompt,
			MotionSuggestion:   motionFor(sc),
			DurationSec:        durationFor(sc),
		})
	}
	return col
}

func hashBase(release string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(release))
	return int(h.Sum32())
}

// buildPrompt composes the scene's SDXL prompt from the full storyboard scene.
func buildPrompt(sc storyboard.Scene, composition string) string {
	return ScenePrompt(sc.Visuals.Description, sc.Type, composition)
}

// ScenePrompt composes a scene's SDXL prompt: the shared style anchor, the
// scene's focal subject (its visual direction, or a grounded metaphor for its
// type when the visual is text/logo-centric — those contradict the shared
// "no text, no logos" negative), and the rotated composition. Exported so the
// video renderer produces the SAME prompts as the storyboard-scenes artifact,
// keeping the spec and the rendered images consistent.
func ScenePrompt(visual, sceneType, composition string) string {
	subject := cleanSubject(visual)
	if subject == "" || mentionsTextOrLogo(subject) {
		subject = metaphorFor(sceneType)
	}
	subject = trimToWords(subject, 45)
	// Frame the subject as a concept to ILLUSTRATE, so SDXL depicts the scene's
	// actual services and their relationships (wordlessly) rather than a generic
	// cityscape. Then the composition (framing) + the cinematic quality suffix.
	prompt := SharedStylePrompt + " Illustrate this concept as cloud-service iconography and flows: " + subject + "."
	// Ground the imagery in the SPECIFIC services the scene names, so the picture
	// matches the content — each rendered as its own distinct, recognizable node
	// (wordless; the shapes carry meaning, never labels).
	if svc := detectServiceNodes(visual); svc != "" {
		prompt += " Feature these as distinct, recognizable service nodes connected by directional event-flow arrows (shapes only, absolutely no text labels): " + svc + "."
	}
	if composition != "" {
		prompt += " Composition: " + composition + "."
	}
	prompt += " " + SharedQualitySuffix + "."
	return prompt
}

// serviceNodes maps recognizable architecture components to a short, evocative
// visual description SDXL can render wordlessly. Detection is on the scene's own
// text, so the picture depicts the exact services under discussion.
var serviceNodes = []struct{ key, node string }{
	{"eventbridge", "an event-bus hub fanning out event particles"},
	{"step functions", "a branching state-machine graph"},
	{"step function", "a branching state-machine graph"},
	{"lambda", "small serverless function cubes"},
	{"bedrock", "a cloud inference engine node"},
	{"cloudwatch", "a convergence/observability node collecting light-lines"},
	{"dynamodb", "a fast key-value store node"},
	{"s3", "a glowing object-storage cylinder with asset cards"},
	{"efs", "a central shared-storage disc"},
	{"sqs", "a message-queue pipe with buffered tokens"},
	{"ec2", "an on-demand compute instance block"},
	{"iam", "a security boundary ring enclosing isolated zones"},
	{"api gateway", "an entry gateway node"},
	{"ollama", "a compact local-inference engine node"},
	{"n8n", "a chain of automation workflow nodes"},
	{"github", "a source-repository node emitting a commit particle"},
	{"webhook", "a triggering pulse into the system"},
	{"claude", "an AI reasoning node"},
}

// detectServiceNodes returns a comma-joined list of wordless node descriptions
// for the architecture components named in text (in first-appearance order,
// de-duplicated), or "" when none are recognized.
func detectServiceNodes(text string) string {
	l := strings.ToLower(text)
	type hit struct {
		idx  int
		node string
	}
	var hits []hit
	seen := map[string]bool{}
	for _, s := range serviceNodes {
		if i := strings.Index(l, s.key); i >= 0 && !seen[s.node] {
			seen[s.node] = true
			hits = append(hits, hit{i, s.node})
		}
	}
	if len(hits) == 0 {
		return ""
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].idx < hits[b].idx })
	parts := make([]string, 0, len(hits))
	for _, h := range hits {
		parts = append(parts, h.node)
	}
	if len(parts) > 5 { // keep the composition legible
		parts = parts[:5]
	}
	return strings.Join(parts, ", ")
}

// cleanSubject strips storyboard boilerplate ("A supporting visual for: …" and a
// trailing "Assets: …" editing note) so the SDXL subject is the scene's actual
// concept (its title names the real services/ideas), not meta-noise.
func cleanSubject(visual string) string {
	s := strings.TrimSpace(visual)
	for _, p := range []string{"A supporting visual for:", "A supporting visual for"} {
		s = strings.TrimSpace(strings.TrimPrefix(s, p))
	}
	if i := strings.Index(s, "Assets:"); i >= 0 {
		s = s[:i]
	}
	return strings.Trim(strings.TrimSpace(s), ".: ")
}

// ChooseComposition returns the rotated cinematic composition for a scene index
// within a release — the same rotation the storyboard-scenes artifact uses.
func ChooseComposition(release string, index int) string {
	return compositions[(hashBase(release)+index)%len(compositions)]
}

// mentionsTextOrLogo reports whether a visual description leans on on-screen text
// or logos, which SDXL should not render for these illustrations.
func mentionsTextOrLogo(s string) bool {
	l := strings.ToLower(s)
	for _, kw := range []string{"logo", "title card", "release tag", "caption", "headline", "on-screen text", "text overlay", "wordmark"} {
		if strings.Contains(l, kw) {
			return true
		}
	}
	return false
}

// stripMarkdown removes leading Markdown structural markers (heading #, blockquote
// >, list bullets) so a narration line reads as plain prose.
func stripMarkdown(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "#>*-•\t ")
	return strings.TrimSpace(s)
}

// purposeFor maps a storyboard scene type to a one-word narrative purpose.
func purposeFor(sceneType string) string {
	switch strings.ToLower(sceneType) {
	case "introduction":
		return "Hook"
	case "problem":
		return "Problem"
	case "architecture", "diagram":
		return "Architecture"
	case "cloudformation":
		return "Infrastructure as code"
	case "repository":
		return "Repository structure"
	case "implementation":
		return "Implementation"
	case "results":
		return "Results"
	case "lessons":
		return "Lesson"
	case "conclusion":
		return "Conclusion"
	default:
		return "Concept"
	}
}

// metaphorFor is the grounded visual metaphor used when a scene has no visuals
// description — a concept, never an AWS logo collage.
func metaphorFor(sceneType string) string {
	switch strings.ToLower(sceneType) {
	case "introduction":
		return "A single glowing entry node opening into a layered cloud system, event-driven and orchestrated."
	case "problem":
		return "A constricted flow bottlenecking through a narrow gate, slow and inefficient, tension in the composition."
	case "architecture", "diagram":
		return "Layered infrastructure planes connected by directional event arrows, a serverless control plane above an on-demand compute plane."
	case "cloudformation":
		return "Declarative infrastructure blueprints assembling into provisioned resources, immutable and versioned."
	case "implementation":
		return "A build pipeline assembling artifacts left to right into a captured, tagged immutable image."
	case "results":
		return "A fast, clean event flow completing end to end, efficient and orchestrated, positive momentum."
	case "conclusion":
		return "A complete, calm cloud-native system at rest, orchestrated and observable, resolved composition."
	default:
		return "An abstract cloud-native automation flow, event-driven and layered, clean and orchestrated."
	}
}

// cameraFor normalises the storyboard scene's camera direction to the
// FFmpeg-friendly vocabulary, rotating through the set when none is given.
func cameraFor(sc storyboard.Scene, seed int) string {
	d := strings.ToLower(strings.TrimSpace(sc.Camera.Direction))
	switch {
	case strings.Contains(d, "push") || strings.Contains(d, "zoom in"):
		return "slow push in"
	case strings.Contains(d, "pull") || strings.Contains(d, "zoom out"):
		return "slow pull out"
	case strings.Contains(d, "pan left") || strings.Contains(d, "left"):
		return "left pan"
	case strings.Contains(d, "pan right") || strings.Contains(d, "right"):
		return "right pan"
	case strings.Contains(d, "orbit"):
		return "orbit move"
	case strings.Contains(d, "parallax"):
		return "parallax drift"
	}
	return cameraMoves[seed%len(cameraMoves)]
}

// motionFor suggests an animation, derived from the scene's first animation cue
// or its type.
func motionFor(sc storyboard.Scene) string {
	if len(sc.Animations) > 0 {
		switch t := strings.ToLower(sc.Animations[0].Type); {
		case strings.Contains(t, "diagram") || strings.Contains(t, "draw") || strings.Contains(t, "arrow"):
			return "Slow infrastructure reveal"
		case strings.Contains(t, "highlight") || strings.Contains(t, "pulse"):
			return "Node pulse animation"
		case strings.Contains(t, "code") || strings.Contains(t, "typing"):
			return "Layered depth animation"
		case strings.Contains(t, "fade"):
			return "Ken Burns zoom in"
		}
	}
	switch strings.ToLower(sc.Type) {
	case "problem":
		return "Ken Burns zoom in"
	case "architecture", "diagram":
		return "Event flow animation"
	case "results", "conclusion":
		return "Parallax foreground drift"
	default:
		return "Ken Burns zoom out"
	}
}

// durationFor returns the scene's recommended seconds, defaulting to 5.
func durationFor(sc storyboard.Scene) int {
	if sc.Duration.RecommendedSec > 0 {
		return sc.Duration.RecommendedSec
	}
	if sc.Duration.MinSec > 0 {
		return sc.Duration.MinSec
	}
	return 5
}

// narrationLead returns the first real narration sentence, skipping any leading
// Markdown heading/blank lines the storyboard narration may carry.
func narrationLead(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		return firstSentence(stripMarkdown(ln))
	}
	return firstSentence(stripMarkdown(s))
}

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".!?"); i >= 0 {
		return strings.TrimSpace(s[:i+1])
	}
	return s
}

func trimToWords(s string, n int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	f := strings.Fields(s)
	if len(f) <= n {
		if !strings.HasSuffix(s, ".") {
			return s + "."
		}
		return s
	}
	return strings.Join(f[:n], " ") + "."
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Markdown renders the collection as the storyboard-scenes.md artifact.
func (col SceneCollection) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Storyboard Scenes: %s\n\n", firstNonEmpty(col.SourceTitle, col.Repository))
	fmt.Fprintf(&b, "_%s · release %s · %d scenes · SDXL-ready_\n\n", col.Repository, col.Release, len(col.Scenes))

	b.WriteString("```yaml\nimage_model:\n  provider: replicate\n")
	fmt.Fprintf(&b, "  model: %s\n```\n\n", col.ModelTarget)

	b.WriteString("## Shared Visual Style\n\n```text\n")
	b.WriteString(SharedStylePrompt)
	b.WriteString("\n```\n\n### Quality Suffix (appended to every prompt)\n\n```text\n")
	b.WriteString(SharedQualitySuffix)
	b.WriteString("\n```\n\n### Shared Negative Prompt\n\n```text\n")
	b.WriteString(SharedNegativePrompt)
	b.WriteString("\n```\n\n")

	for _, s := range col.Scenes {
		fmt.Fprintf(&b, "---\n\n## Scene %02d\n\n", s.Number)
		fmt.Fprintf(&b, "### Purpose\n%s\n\n", s.Purpose)
		fmt.Fprintf(&b, "### Narration Alignment\n%s\n\n", firstNonEmpty(s.NarrationAlignment, "—"))
		fmt.Fprintf(&b, "### Camera Direction\n%s\n\n", s.CameraDirection)
		fmt.Fprintf(&b, "### SDXL Prompt\n\n```text\n%s\n```\n\n", s.SDXLPrompt)
		fmt.Fprintf(&b, "### Negative Prompt\n\n```text\n%s\n```\n\n", s.NegativePrompt)
		fmt.Fprintf(&b, "### Motion Suggestion\n%s\n\n", s.MotionSuggestion)
		fmt.Fprintf(&b, "### Duration\n%d seconds\n\n", s.DurationSec)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}
