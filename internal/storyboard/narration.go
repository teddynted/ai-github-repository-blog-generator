package storyboard

import (
	"context"
	"fmt"
	"strings"
)

// narration returns spoken voice-over for a scene. It builds a deterministic
// draft from the section prose (grounded, never invented) and, when a Model is
// configured, asks it to polish the draft into natural spoken narration — never
// to add facts. On any model error it falls back to the draft.
func (g *Generator) narration(ctx context.Context, sec section, typ, nextTitle, prior string) string {
	draft := narrationDraft(sec.Body, typ, sec.Title)
	if g.Model == nil || strings.TrimSpace(draft) == "" {
		return draft
	}
	out, err := g.Model.Generate(ctx, narrationPrompt(sec.Title, typ, nextTitle, prior, draft))
	if err != nil {
		return draft
	}
	if r := collapse(stripNarrationPreamble(out)); r != "" {
		return r
	}
	return draft
}

// fitToSceneBudget trims narration to the words that can be spoken within a
// single scene's maximum allotted time (sceneMaxSec) at the pacing rate. It cuts
// on sentence boundaries only — spoken narration is never clipped mid-sentence —
// so the estimated speech time never exceeds the scene's allocated duration.
// This is the hard timing contract the voice-over relies on: with narration
// bounded to the slot, the duration clamp never bites and no scene is ever
// reported "over".
func fitToSceneBudget(narration string, wordsPerSecond float64) string {
	if wordsPerSecond <= 0 {
		wordsPerSecond = defaultWordsPerSecond
	}
	budget := int(float64(sceneMaxSec) * wordsPerSecond)
	if budget < 1 || wordCount(narration) <= budget {
		return narration
	}
	// A very large sentence cap: keep whole sentences, bounded only by the word
	// budget (firstSentences always keeps at least the first sentence intact).
	return firstSentences(narration, 1<<30, budget)
}

// stripNarrationPreamble removes the assistant-style framing that small models
// prepend to voice-over ("Here is a rewritten version of the draft in 2-3
// sentences:") and the surrounding quotes they often add, leaving only the
// spoken narration.
func stripNarrationPreamble(s string) string {
	s = strings.TrimSpace(s)
	// Drop a leading meta clause that ends at the first ':' when it reads as the
	// model narrating what it did rather than the narration itself.
	if i := strings.IndexByte(s, ':'); i > 0 && i < 160 {
		head := strings.ToLower(s[:i])
		for _, m := range []string{"here is", "here's", "here are", "sure", "certainly",
			"rewritten", "rewrite", "revised", "version of", "as requested",
			"spoken sentence", "spoken narration", "narration", "the draft"} {
			if strings.Contains(head, m) {
				s = strings.TrimSpace(s[i+1:])
				break
			}
		}
	}
	// Strip a single pair of wrapping straight or curly quotes.
	s = strings.TrimSpace(s)
	for _, q := range []struct{ open, close string }{{"\"", "\""}, {"'", "'"}, {"“", "”"}} {
		if strings.HasPrefix(s, q.open) && strings.HasSuffix(s, q.close) && len(s) > len(q.open)+len(q.close) {
			s = strings.TrimSpace(s[len(q.open) : len(s)-len(q.close)])
			break
		}
	}
	return strings.TrimSpace(s)
}

// baselineComponentTerms name repository-wide runtime components that are not the
// subject of a typical release. Narration sentences mentioning them are dropped so
// a release-scoped scene (e.g. "The Existing Architecture" for an AMI release)
// narrates the release's own components — EventBridge Scheduler, Lambda, EC2,
// CloudWatch — instead of the baseline AI platform. Mirrors the archspec filter of
// the same name; keep the two lists in sync. Only specific product names are
// listed, never generic words a future release might legitimately be about.
var baselineComponentTerms = []string{"claw", "openclaw", "ollama", "bedrock", "n8n", "kafka", "efs"}

// scopeToRelease drops whole sentences that mention a baseline platform component,
// keeping narration on the release's subject. Returns "" if nothing survives, so
// the caller falls back to a title-based line.
func scopeToRelease(text string) string {
	var kept []string
	for _, s := range splitSentences(text) {
		low := strings.ToLower(s)
		drop := false
		for _, t := range baselineComponentTerms {
			if containsWord(low, t) {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, s)
		}
	}
	return strings.TrimSpace(strings.Join(kept, " "))
}

// splitSentences splits text into sentences on ., ! and ? boundaries.
func splitSentences(s string) []string {
	s = strings.Join(strings.Fields(s), " ")
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if isSentenceBoundary(s, i) {
			if seg := strings.TrimSpace(s[start : i+1]); seg != "" {
				out = append(out, seg)
			}
			start = i + 1
		}
	}
	if tail := strings.TrimSpace(s[start:]); tail != "" {
		out = append(out, tail)
	}
	return out
}

// containsWord reports whether text contains word bounded by non-word characters
// (letters/digits are word characters). Both arguments must be lowercase.
func containsWord(text, word string) bool {
	isWord := func(b byte) bool { return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' }
	for idx := 0; ; {
		i := strings.Index(text[idx:], word)
		if i < 0 {
			return false
		}
		i += idx
		before := i == 0 || !isWord(text[i-1])
		after := i+len(word) >= len(text) || !isWord(text[i+len(word)])
		if before && after {
			return true
		}
		idx = i + 1
	}
}

// narrationDraft produces grounded narration from the section prose, with a
// sensible fallback line when the section has no prose (e.g. a diagram-only
// section). Baseline platform components are scoped out so the draft stays on the
// release's subject.
// isDeepTechnical reports whether a scene type warrants a fuller narration —
// implementation and infrastructure scenes carry the concrete detail (identifiers,
// tags, manifests) that a 3-sentence draft would truncate.
func isDeepTechnical(typ string) bool {
	return typ == "implementation" || typ == "cloudformation"
}

func narrationDraft(body, typ, title string) string {
	sentences, maxW := 3, 60
	if isDeepTechnical(typ) {
		sentences, maxW = 5, 100
	}
	p := scopeToRelease(prose(body))
	if d := firstSentences(p, sentences, maxW); d != "" {
		return d
	}
	switch typ {
	case "diagram", "architecture":
		return fmt.Sprintf("Here's the architecture at a glance: %s.", strings.ToLower(title))
	default:
		return "Let's look at " + strings.ToLower(cleanInline(title)) + "."
	}
}

// visualHint returns a one-line steer so the narration matches what is on screen
// for the scene's type. Empty when the type has no distinctive visual.
func visualHint(typ string) string {
	switch typ {
	case "architecture", "diagram":
		return "On screen is the architecture diagram — describe how the components interact and the flow of data and control between them.\n"
	case "implementation", "cloudformation", "repository":
		return "On screen is code or a repository view — talk about the actual scripts, manifests, tags, identifiers, or changed files, not abstractions.\n"
	case "results":
		return "On screen are outcome graphics — talk about the concrete, measurable operational benefits.\n"
	default:
		return ""
	}
}

func narrationPrompt(title, typ, nextTitle, prior, draft string) string {
	target := "2–4 short, spoken sentences (about 35–55 words total"
	sceneHint := ""
	if isDeepTechnical(typ) {
		// Deep-technical scenes carry the concrete detail — give them room to name
		// each mechanism (version resolution, duplicate checks, manifest, tagging).
		target = "3–5 short, spoken sentences (about 55–85 words total"
		sceneHint = "This is a deep-technical scene: briefly cover EACH distinct mechanism the DRAFT names — the viewer should come away knowing the full set of steps, not just the first one explained at length. Give each mechanism a sentence or clause; do not dwell on only the opening point.\n"
	}
	if typ == "conclusion" {
		// A conference-style close that zooms out to the platform: problem → pattern
		// → what it enables at the platform level, ending on one memorable line.
		sceneHint = "This is the closing scene: land it in three quick beats — the problem this solved, the reusable engineering pattern it demonstrates, and what it now enables at the PLATFORM level (separating the control plane from the compute plane, making infrastructure reproducible and observable, and enabling faster recovery and scaling for the event-driven platform). End on ONE short, memorable, forward-looking line. Do not name specific inference products.\n"
	}
	if strings.TrimSpace(prior) == "" && typ != "conclusion" {
		// Opening scene: hook like an incident review, not an introduction.
		sceneHint = "This is the OPENING scene: create immediate technical tension in the first two sentences — what breaks or becomes expensive, why startup latency matters operationally, and why it bites an event-driven platform that starts hosts on demand. Open like an incident review or architecture retrospective; do NOT start with \"This matters\", \"The platform keeps\", or \"Let's look at\".\n"
	}
	bridge := ""
	if strings.TrimSpace(nextTitle) != "" {
		bridge = fmt.Sprintf("The next scene is %q. When it changes focus from this one, end with exactly one short "+
			"bridging sentence that hands the viewer off to it — a natural lead-in, not a summary or a teaser. If the next "+
			"scene simply continues this exact point, add no bridge.\n", nextTitle)
	}
	dedup := ""
	if p := strings.TrimSpace(prior); p != "" {
		// Keep only the most recent context so the prompt stays bounded.
		dedup = fmt.Sprintf("EARLIER SCENES ALREADY SAID (do NOT repeat these points; where this scene's draft overlaps "+
			"them, cover only the NEW angle this scene adds — for example a runtime-problem scene, a one-line mental "+
			"model, and a step-by-step walkthrough must each say something distinct): %s\n", capWords(p, 220))
	}
	return fmt.Sprintf(
		"You are a senior AWS platform engineer narrating one scene of a technical explainer video for an "+
			"audience of software and DevOps engineers — the tone of a re:Invent speaker or a technical YouTube educator.\n"+
			"Scene: %q (type: %s).\n"+
			"%s%s%s%s\n"+
			"Rewrite the DRAFT below into %s, roughly 12–18 words per sentence). Be clear, confident, conversational, and "+
			"technically precise. Prefer present tense and active voice; keep each sentence to a single idea rather than "+
			"long compound clauses.\n"+
			"Vary your sentence openings for spoken rhythm: do not begin consecutive sentences with the same word or the "+
			"same subject (for example repeated \"The pipeline\", \"The builder\", \"The host\", \"This\", or \"That\"), "+
			"and never start a sentence with \"So\", \"Then\", or \"Think of it as\". Keep EVERY sentence under 20 spoken "+
			"words — if one runs longer, split it into two so it reads in a single breath.\n"+
			"Vary distinctive phrasing across sentences: introduce a signature term once (a central verb or noun), then "+
			"prefer natural technical variations instead of leaning on the same word every sentence. Do not reuse the exact "+
			"same distinctive phrase that EARLIER SCENES already used (below) — say it a different way.\n"+
			"Ground every claim in the DRAFT: use ONLY the facts, AWS services, and mechanisms it states — never invent "+
			"features, numbers, or components, and never substitute a different AWS service for the one named (for example, "+
			"do not say ECS when the draft says EC2).\n"+
			"Stay terminology-consistent: reuse the DRAFT's own names for each thing, and do not alternate between "+
			"\"machine\", \"host\", \"server\", and \"instance\" for the same component.\n"+
			"Do NOT use autogenerated filler, absolute overclaims, dramatic, or marketing phrasing. Avoid, for example: "+
			"\"This approach\", \"several key benefits\", \"the system's capacity is depleted\", \"guards are deployed\", "+
			"\"no two starts produced the same machine\", \"a critical phase of production\", \"streamline the "+
			"experience\", \"at the center\", \"in kind\", \"everything downstream stays the same\", \"the payoff is\", "+
			"\"that marks this as\", \"applied broadly\", \"hides choices worth a closer look\", and a sentence "+
			"starting with \"Now\". State what actually happens, in plain engineering language.\n"+
			"Output ONLY the spoken sentences — no preamble, no quotation marks, no scene labels, and no framing such as "+
			"\"Here is\" or \"rewritten version\". Begin directly with the first spoken word.\n\n"+
			"DRAFT:\n%s",
		title, typ, visualHint(typ), bridge, sceneHint, dedup, target, draft,
	)
}

// planVisuals describes what fills the frame for a scene (deterministic, tied to
// the chosen assets and diagrams).
func planVisuals(typ, title string, assets []string, diagrams []DiagramRef, repeat bool) Visuals {
	v := Visuals{Style: "clean, modern, developer-focused", Background: backgroundFor(typ)}
	switch typ {
	case "introduction":
		v.Description = "Animated title card with the repository logo and release tag; brief motion-graphic intro."
	case "problem":
		v.Description = "A before/after timeline dramatising the operational cost — animate the slow path filling up; keep on-screen text light so the narration carries it."
	case "constraint":
		v.Description = "An animated boot/provisioning timeline: package installs streaming from live repositories, with two runs diverging to show the non-determinism. Highlight the delta."
	case "solution":
		v.Description = "The build-pipeline reveal, assembling left to right (scripts → artifact store → builder → manifest → captured, tagged artifact). Camera: a slow lateral track along the pipeline."
	case "decisions":
		v.Description = "A state-transition overlay contrasting the wrong path with the right one (e.g. skipped vs. reset first-boot state), each decision resolving on screen as narration names it."
	case "tradeoffs":
		v.Description = "A lifecycle animation: versioned artifacts stacking, each with its own snapshot, a storage meter ticking up, and a rebuild-cadence clock — the costs made visible."
	case "future":
		v.Description = "A capability unlock: the fast path enabling an interruption-and-recovery cycle — reclaim, relaunch from the artifact, back in service — animated end to end."
	case "architecture", "diagram":
		switch {
		case len(diagrams) > 0 && repeat:
			v.Description = fmt.Sprintf("Recall the architecture diagram (%s) already on screen — do NOT rebuild it. Pan/zoom to the region this scene discusses and re-highlight only its nodes.", diagrams[0].Source)
		case len(diagrams) > 0:
			v.Description = fmt.Sprintf("The architecture diagram (%s): build the graph edge by edge, lower-third each AWS service as narration names it, and pulse the single most important cross-plane hand-off.", diagrams[0].Source)
		default:
			v.Description = "An architecture canvas that builds the components in one at a time, with a lower-third AWS-service label appearing as each is introduced."
		}
	case "cloudformation":
		v.Description = "A scrolling CloudFormation template / change-set view, lower-thirding each resource type as it scrolls past and highlighting the lines the narration cites."
	case "repository":
		v.Description = "A screen recording of the repository diff: the file tree, then the changed files, zooming the key hunks the narration names."
	case "implementation":
		v.Description = "A terminal or editor screen recording of the key mechanism — scroll or type the real identifiers, tags, and config, highlighting each as the narration reaches it."
	case "results":
		v.Description = "Outcome graphics: metric tiles plus a before/after comparison, animating the delta between the two states."
	case "lessons":
		v.Description = "Clean slides listing practical guidance and how to use or extend the feature."
	case "conclusion":
		v.Description = "Closing title card recapping the reusable pattern with a subtle call to action."
	default:
		v.Description = "A supporting visual for: " + title + "."
	}
	if len(assets) > 0 {
		v.Description += " Assets: " + strings.Join(assets, ", ") + "."
	}
	return v
}

func backgroundFor(typ string) string {
	switch typ {
	case "cloudformation", "repository", "implementation":
		return "dark code-editor theme"
	case "architecture", "diagram":
		return "light diagram canvas"
	default:
		return "brand gradient"
	}
}
