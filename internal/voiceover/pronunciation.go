package voiceover

import "sort"

// lexicon is the curated pronunciation guide for the technical terms this
// project's content actually uses. Entries provide a human-readable respelling
// plus an SSML say-as hint so both narrators and TTS engines handle the term
// correctly. It is intentionally deterministic and extensible — add a term here
// and every downstream script picks it up.
//
// SayAs values map to SSML <say-as interpret-as="…">:
//   - "spell-out"  → read letter by letter (acronyms: AWS, JSON, YAML, IAM)
//   - "characters" → same, for short sym/version tokens
//   - "as-written" → read as a normal word (CloudFormation, Bedrock, Ollama)
var lexicon = []Pronunciation{
	{Term: "AWS", Phonetic: "A-W-S", SayAs: "spell-out", Note: "Never say \"aws\" as a word."},
	{Term: "CloudFormation", Phonetic: "kloud-for-MAY-shun", SayAs: "as-written"},
	{Term: "API Gateway", Phonetic: "A-P-I GATE-way", SayAs: "as-written", Note: "Spell out API, then \"Gateway\"."},
	{Term: "Lambda", Phonetic: "LAM-duh", SayAs: "as-written"},
	{Term: "DynamoDB", Phonetic: "die-NAM-oh-D-B", SayAs: "as-written"},
	{Term: "EventBridge", Phonetic: "ee-VENT-bridge", SayAs: "as-written"},
	{Term: "SQS", Phonetic: "S-Q-S", SayAs: "spell-out"},
	{Term: "Amazon SQS", Phonetic: "AM-uh-zon S-Q-S", SayAs: "as-written"},
	{Term: "EC2", Phonetic: "E-C-two", SayAs: "as-written"},
	{Term: "S3", Phonetic: "S-three", SayAs: "as-written"},
	{Term: "IAM", Phonetic: "I-A-M", SayAs: "spell-out"},
	{Term: "Bedrock", Phonetic: "BED-rock", SayAs: "as-written"},
	{Term: "Amazon Polly", Phonetic: "AM-uh-zon PALL-ee", SayAs: "as-written"},
	{Term: "Polly", Phonetic: "PALL-ee", SayAs: "as-written"},
	{Term: "ElevenLabs", Phonetic: "ee-LEV-en-labs", SayAs: "as-written"},
	{Term: "Azure", Phonetic: "AZH-er", SayAs: "as-written"},
	{Term: "Ollama", Phonetic: "oh-LAH-mah", SayAs: "as-written"},
	{Term: "Claude", Phonetic: "klawd", SayAs: "as-written"},
	{Term: "Qwen", Phonetic: "KWEN", SayAs: "as-written"},
	{Term: "GitHub", Phonetic: "GIT-hub", SayAs: "as-written"},
	{Term: "Mermaid", Phonetic: "MER-mayd", SayAs: "as-written"},
	{Term: "JSON", Phonetic: "JAY-son", SayAs: "as-written"},
	{Term: "YAML", Phonetic: "YAM-ul", SayAs: "as-written"},
	{Term: "Markdown", Phonetic: "MARK-down", SayAs: "as-written"},
	{Term: "Go", Phonetic: "goh", SayAs: "as-written", Note: "The Go programming language."},
	{Term: "Golang", Phonetic: "GO-lang", SayAs: "as-written"},
	{Term: "TTS", Phonetic: "T-T-S", SayAs: "spell-out"},
	{Term: "CLI", Phonetic: "C-L-I", SayAs: "spell-out"},
	{Term: "API", Phonetic: "A-P-I", SayAs: "spell-out"},
	{Term: "HMAC", Phonetic: "AYTCH-mac", SayAs: "as-written"},
	{Term: "SSH", Phonetic: "S-S-H", SayAs: "spell-out"},
	{Term: "SDK", Phonetic: "S-D-K", SayAs: "spell-out"},
	{Term: "SemVer", Phonetic: "SEM-ver", SayAs: "as-written"},
	{Term: "Kubernetes", Phonetic: "koo-ber-NET-eez", SayAs: "as-written"},
	{Term: "nginx", Phonetic: "EN-jin-ex", SayAs: "as-written"},
	{Term: "PostgreSQL", Phonetic: "POST-gres-Q-L", SayAs: "as-written"},
	{Term: "OAuth", Phonetic: "OH-auth", SayAs: "as-written"},
	{Term: "webhook", Phonetic: "WEB-hook", SayAs: "as-written"},
	{Term: "OpenAI", Phonetic: "OH-pen-A-I", SayAs: "as-written"},
}

// planPronunciation returns the pronunciation entries for the terms that
// actually appear in the given text, sorted longest-term-first so multi-word
// terms ("API Gateway", "Amazon SQS") win over their sub-tokens and each term is
// reported once. Deterministic and grounded — no term is emitted unless present.
func planPronunciation(text string) []Pronunciation {
	entries := append([]Pronunciation(nil), lexicon...)
	// Longest term first: a match on "API Gateway" suppresses a later "API".
	sort.SliceStable(entries, func(i, j int) bool {
		return len(entries[i].Term) > len(entries[j].Term)
	})

	var out []Pronunciation
	claimed := map[string]bool{} // lowercased terms already reported for this text
	for _, e := range entries {
		if claimed[lower(e.Term)] {
			continue
		}
		if !containsWord(text, e.Term) {
			continue
		}
		// Suppress sub-tokens of an already-claimed multi-word term.
		if subsumed(e.Term, claimed) {
			continue
		}
		claimed[lower(e.Term)] = true
		out = append(out, e)
	}
	// Stable, human-friendly ordering in the final script: alphabetical by term.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Term < out[j].Term })
	return out
}

// subsumed reports whether term is a whole-word component of any already-claimed
// multi-word term (e.g. "API" within "API Gateway").
func subsumed(term string, claimed map[string]bool) bool {
	for c := range claimed {
		if c != lower(term) && containsWord(c, term) {
			return true
		}
	}
	return false
}

func lower(s string) string { return collapseLower(s) }

func collapseLower(s string) string {
	b := []byte(collapse(s))
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
