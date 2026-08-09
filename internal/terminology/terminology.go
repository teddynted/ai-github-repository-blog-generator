// Package terminology enforces protected-identifier integrity across the content
// suite. blog.md is the canonical technical specification; every downstream
// artifact (storyboard, voiceover, video scripts, captions, social posts) must
// reproduce protected identifiers — repository component names and AWS service
// names — VERBATIM, never renamed, split, re-cased, or replaced by a synonym.
//
// The pipeline has no ASR/transcription stage: captions come from scene titles
// and narration is Amazon Polly TTS of storyboard text, so every surface is
// already text-derived from the blog. The remaining hallucination vector is a
// model paraphrasing a protected term (e.g. "claw" -> "Claude", "n8n" -> "N8N",
// "EventBridge" -> "Event Bridge"). This package provides a deterministic
// registry + a high-precision violation scanner that a validation gate can run
// on any artifact, plus a prompt clause generators inject to prevent the drift.
package terminology

import (
	"regexp"
	"sort"
	"strings"
)

// Term is a protected identifier that must appear verbatim in derived content.
// Canonical is the exact required form. Allowed are additional acceptable
// spellings (e.g. "AWS Lambda" for "Lambda"). Forbidden are known corruptions —
// synonyms or mis-expansions — that must never appear.
type Term struct {
	Canonical string
	Allowed   []string
	Forbidden []string
}

// Violation is one protected-term breach found in an artifact.
type Violation struct {
	Canonical string // the term that was breached, e.g. "claw"
	Found     string // the offending text, e.g. "Claude"
	Reason    string // "forbidden variant" | "split compound" | "mis-cased"
}

// Registry is the set of protected terms for a release.
type Registry struct {
	terms []Term
}

// base is the platform's curated protected identifiers. Forbidden lists the
// corruptions actually seen (or plausible) from a paraphrasing model; the case
// scanner adds the rest for mixed-case identifiers automatically.
func base() []Term {
	return []Term{
		// OpenClaw is the orchestrator's real name. In the architecture graph its
		// node-id is the shorthand "claw" (like eb=EventBridge, lam=Lambda) — fine
		// INSIDE a Mermaid/code block, never in prose. Violations strips code first,
		// so "claw" surviving into narration/captions is flagged against OpenClaw.
		{Canonical: "OpenClaw", Forbidden: []string{"claw", "Claw", "CLAW", "Claude", "Clawd", "clawed", "Open Claw", "openclaw"}},
		{Canonical: "Ollama", Forbidden: []string{"Olama", "Ollamma", "OLama"}},
		{Canonical: "n8n", Forbidden: []string{"N8N", "N8n", "n8N", "Node8", "nadn"}},
		{Canonical: "EventBridge", Allowed: []string{"Amazon EventBridge"}, Forbidden: []string{"Event Bridge", "EventBus", "Event-Bridge"}},
		{Canonical: "Lambda", Allowed: []string{"AWS Lambda"}, Forbidden: []string{"Lamda", "Lambada"}},
		{Canonical: "Amazon Bedrock", Allowed: []string{"Bedrock"}, Forbidden: []string{"Bed Rock", "BedRock", "Amazon BedRock", "Bed-Rock"}},
		{Canonical: "EFS", Allowed: []string{"Amazon EFS"}, Forbidden: []string{"E F S", "Elastic File System store"}},
		{Canonical: "CloudWatch", Allowed: []string{"Amazon CloudWatch"}, Forbidden: []string{"Cloud Watch", "Cloudwatch", "Cloud-Watch"}},
		{Canonical: "IAM", Allowed: []string{"AWS IAM"}, Forbidden: []string{"I A M"}},
		{Canonical: "S3", Allowed: []string{"Amazon S3"}, Forbidden: []string{"S 3"}},
		// NB: "GitHub" is deliberately NOT protected here — it appears lowercased in
		// legitimate URLs (github.com) and handles, which a case check would flag.
	}
}

// Base returns the curated registry — the protected identifiers common to this
// platform, independent of any single release. It is the registry a static
// validation gate uses when it has no Release Context to hand.
func Base() *Registry { return &Registry{terms: base()} }

// Derive returns the curated registry augmented with the release's OWN AWS
// services, component names, and diagram node labels (each added verbatim and
// case-sensitive) so release-specific identifiers are protected too. Inputs are
// free-form strings from the Release Context; blanks and duplicates are ignored,
// and any that merely re-spell a curated term are skipped.
func Derive(awsServices, components, nodes []string) *Registry {
	r := &Registry{terms: base()}
	known := map[string]bool{}
	for _, t := range r.terms {
		known[strings.ToLower(t.Canonical)] = true
		for _, a := range t.Allowed {
			known[strings.ToLower(a)] = true
		}
	}
	for _, s := range concat(awsServices, components, nodes) {
		s = strings.TrimSpace(s)
		// Skip blanks, single characters, and anything already covered.
		if len(s) < 2 || known[strings.ToLower(s)] {
			continue
		}
		known[strings.ToLower(s)] = true
		r.terms = append(r.terms, Term{Canonical: s})
	}
	return r
}

// Phrases returns the multi-word protected forms (e.g. "Amazon Bedrock", "AWS
// Lambda", "Amazon CloudWatch") — the compound identifiers a caption line must
// never split across a line break.
func (r *Registry) Phrases() []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range r.terms {
		for _, f := range append([]string{t.Canonical}, t.Allowed...) {
			if len(strings.Fields(f)) > 1 && !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	return out
}

// Terms returns the protected canonical forms, sorted, for prompts and reports.
func (r *Registry) Terms() []string {
	out := make([]string, 0, len(r.terms))
	for _, t := range r.terms {
		out = append(out, t.Canonical)
	}
	sort.Strings(out)
	return out
}

// Violations scans text and returns every protected-term breach: a forbidden
// variant/synonym, a space-split camelCase compound, or a mis-cased mixed-case
// identifier. It is high-precision by design — it never flags a term that is
// merely absent, and it avoids sentence-initial false positives by only
// case-checking identifiers that contain both upper- and lower-case letters
// (so a lower-case term like "claw" is guarded by its Forbidden list, never by
// capitalization, which is legitimate at a sentence start).
func (r *Registry) Violations(text string) []Violation {
	// Protected-term rules apply to PROSE, not diagram code: a Mermaid/code block
	// legitimately uses short node-ids (claw, eb, lam, gh). Strip fenced and inline
	// code first so those never register as violations.
	text = stripCode(text)
	var out []Violation
	seen := map[string]bool{}
	add := func(v Violation) {
		key := v.Canonical + "\x00" + v.Found + "\x00" + v.Reason
		if !seen[key] {
			seen[key] = true
			out = append(out, v)
		}
	}
	for _, t := range r.terms {
		allowed := allowedForms(t)
		// 1. Forbidden variants / synonyms (case-sensitive, word-bounded).
		for _, f := range t.Forbidden {
			if containsWord(text, f) {
				add(Violation{t.Canonical, f, "forbidden variant"})
			}
		}
		// 2. Split camelCase compound: "EventBridge" -> "Event Bridge".
		if split := splitCamel(t.Canonical); split != "" && containsWord(text, split) {
			add(Violation{t.Canonical, split, "split compound"})
		}
		// 3. Mis-cased mixed-case identifier: any case-insensitive occurrence whose
		// exact text is not an allowed form (e.g. "eventbridge", "OLLAMA").
		if hasUpperAndLower(t.Canonical) {
			for _, m := range foldMatches(text, t.Canonical) {
				// Exempt stylistic ALL-CAPS (thumbnail text, on-screen overlays):
				// "CLOUDWATCH" on a card is deliberate, not a corruption.
				if !allowed[m] && !isAllUpper(m) {
					add(Violation{t.Canonical, m, "mis-cased"})
				}
			}
		}
	}
	return out
}

// PromptClause renders the injectable instruction that tells a generator to
// reproduce these identifiers verbatim. Generators append it to their prompt so
// the drift is prevented at the source, not only caught at the gate.
func (r *Registry) PromptClause() string {
	terms := r.Terms()
	if len(terms) == 0 {
		return ""
	}
	return "PROTECTED TERMS — reproduce these identifiers EXACTLY as written, with the same " +
		"capitalization: " + strings.Join(terms, ", ") + ". Never rename them, never replace one " +
		"with a synonym or a different service, never split a compound name (write \"EventBridge\", " +
		"never \"Event Bridge\"), and never re-spell one from how it sounds (\"n8n\" is never " +
		"\"N8N\"). The orchestrator is \"OpenClaw\"; its diagram node-id \"claw\" is shorthand that " +
		"must NEVER appear in prose — always write \"OpenClaw\". Short forms already shown are fine " +
		"(Lambda, Bedrock, S3); everything else stays byte-for-byte."
}

// allowedForms returns the set of exact spellings that satisfy a term.
func allowedForms(t Term) map[string]bool {
	m := map[string]bool{t.Canonical: true}
	for _, a := range t.Allowed {
		m[a] = true
	}
	return m
}

var (
	nonWordRe    = regexp.MustCompile(`\W`)
	camelSplitRe = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	fencedCodeRe = regexp.MustCompile("(?s)```.*?```")
	inlineCodeRe = regexp.MustCompile("`[^`]*`")
)

// stripCode removes fenced and inline code (Mermaid diagrams, snippets) so the
// short node-ids they legitimately contain are not scanned as prose violations.
func stripCode(text string) string {
	text = fencedCodeRe.ReplaceAllString(text, " ")
	return inlineCodeRe.ReplaceAllString(text, " ")
}

// containsWord reports whether needle occurs in text on word-ish boundaries, so
// "Event Bridge" matches inside a sentence but not inside a larger token. The
// boundary is enforced with case-sensitive literal matching of needle framed by
// non-word chars or string ends.
func containsWord(text, needle string) bool {
	if needle == "" {
		return false
	}
	from := 0
	for {
		i := strings.Index(text[from:], needle)
		if i < 0 {
			return false
		}
		i += from
		if boundaryOK(text, i, i+len(needle)) {
			return true
		}
		from = i + 1
		if from >= len(text) {
			return false
		}
	}
}

// foldMatches returns every case-insensitive, word-bounded occurrence of needle
// in text, as it actually appears (preserving the found casing).
func foldMatches(text, needle string) []string {
	if needle == "" {
		return nil
	}
	lowerText, lowerNeedle := strings.ToLower(text), strings.ToLower(needle)
	var out []string
	from := 0
	for {
		i := strings.Index(lowerText[from:], lowerNeedle)
		if i < 0 {
			return out
		}
		i += from
		end := i + len(needle)
		// Whole-token match, and not part of a kebab-case slug (a hyphen neighbour
		// means a tag/handle like "aws-lambda" or "amazon-cloudwatch", not prose).
		if boundaryOK(text, i, end) && !hyphenAdjacent(text, i, end) {
			out = append(out, text[i:end])
		}
		from = i + 1
		if from >= len(lowerText) {
			return out
		}
	}
}

// hyphenAdjacent reports whether the char immediately before start or after end
// is a hyphen — i.e. the match sits inside a kebab-case slug (a tag or handle).
func hyphenAdjacent(text string, start, end int) bool {
	if start > 0 && text[start-1] == '-' {
		return true
	}
	return end < len(text) && text[end] == '-'
}

// boundaryOK reports whether [start,end) in text is framed by non-word chars or
// string ends — a whole-token match rather than a substring of a larger word.
func boundaryOK(text string, start, end int) bool {
	if start > 0 && !nonWordRe.MatchString(text[start-1:start]) {
		return false
	}
	if end < len(text) && !nonWordRe.MatchString(text[end:end+1]) {
		return false
	}
	return true
}

// splitCamel turns a camelCase compound into its space-split corruption
// ("EventBridge" -> "Event Bridge"), or "" when the term is not camelCase.
func splitCamel(s string) string {
	out := camelSplitRe.ReplaceAllString(s, "$1 $2")
	if out == s {
		return ""
	}
	return out
}

// isAllUpper reports whether s has letters and all of them are upper-case, so a
// deliberately capitalized overlay/thumbnail form is not treated as mis-cased.
func isAllUpper(s string) bool {
	hasLetter := false
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			hasLetter = true
		}
	}
	return hasLetter
}

func hasUpperAndLower(s string) bool {
	var upper, lower bool
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= 'a' && r <= 'z':
			lower = true
		}
	}
	return upper && lower
}

func concat(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}
