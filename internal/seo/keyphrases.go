package seo

import "strings"

// topicAnchors are concept words that mark a phrase as engineering-topic material
// (an optimization/technique/behavior a developer searches for) rather than
// service/infrastructure inventory. A candidate keyphrase from the title/meta is
// kept only if it contains one — so "EC2 startup optimization" qualifies but
// "AWS IAM role" does not.
var topicAnchors = map[string]bool{
	"ami": true, "amis": true, "image": true, "images": true, "imaging": true,
	"startup": true, "boot": true, "boottime": true, "cold": true,
	"provisioning": true, "provision": true, "userdata": true,
	"latency": true, "optimization": true,
	"spot": true, "immutable": true, "cache": true, "caching": true,
	"baked": true, "prebaked": true,
	"deployment": true, "pipeline": true, "orchestration": true,
	"inference": true, "routing": true, "throughput": true, "performance": true,
	"scaling": true, "reduction": true,
	"streaming": true, "indexing": true, "query": true, "concurrency": true,
	"memory": true, "migration": true, "resilience": true,
	"idempotent": true, "idempotency": true, "retry": true, "backpressure": true,
	"warm": true, "snapshot": true, "versioned": true, "rollback": true,
	"ondemand": true,
}

// phraseBreakers split prose into candidate phrase chunks: articles,
// prepositions, conjunctions, and common verbs never belong INSIDE a keyphrase,
// so a run of words between two breakers is one candidate.
var phraseBreakers = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "of": true,
	"to": true, "for": true, "in": true, "on": true, "with": true, "is": true,
	"it": true, "its": true, "this": true, "that": true, "what": true, "why": true,
	"how": true, "using": true, "into": true, "from": true, "by": true, "via": true,
	"as": true, "at": true, "but": true, "so": true, "when": true, "where": true,
	"make": true, "makes": true, "trades": true, "trade": true, "replacing": true,
	"replace": true, "reduce": true, "reducing": true, "eliminate": true,
	"eliminating": true, "build": true, "building": true, "design": true,
	"designing": true, "use": true, "uses": true, "than": true, "then": true,
	"cut": true, "cuts": true, "cutting": true, "add": true, "adds": true,
	"remove": true, "removes": true, "removing": true, "move": true, "moves": true,
	"moving": true, "turn": true, "turns": true, "save": true, "saves": true,
	"saving": true, "keep": true, "keeps": true, "get": true, "gets": true,
	"help": true, "helps": true, "let": true, "lets": true, "without": true,
	"improve": true, "improves": true, "improving": true, "avoid": true, "avoids": true,
}

// phraseTailTrim are trailing adjectives/adverbs that add no search value at the
// end of a keyphrase ("… EC2 startup fast" → "… EC2 startup").
var phraseTailTrim = map[string]bool{
	"fast": true, "faster": true, "slow": true, "slower": true, "easy": true,
	"easier": true, "simple": true, "simpler": true, "better": true, "best": true,
	"quick": true, "quickly": true, "efficiently": true, "reliably": true,
}

// keyphrasesFromText extracts topic keyphrases from prose (the article title and
// meta description). It splits the text into chunks at connector words and
// punctuation, then keeps each 2–4 word content-run that contains a topic anchor
// and survives the primary-keyword noise filters (no services, platform
// concepts, repo fragments). Original casing is preserved ("Pre-Baked Custom
// AMIs"). This lets primary keywords represent the ARTICLE's search intent even
// when the upstream SEO keyword list is pure service inventory.
func keyphrasesFromText(text string, pkg ReleasePackage) []string {
	awsSet := lowerSet(awsServices(pkg))
	var out []string
	seen := map[string]bool{}
	for _, chunk := range splitPhraseChunks(text) {
		phrase := trimPhrase(chunk)
		words := strings.Fields(phrase)
		if len(words) < 2 || len(words) > 4 {
			continue
		}
		lc := strings.ToLower(phrase)
		if !phraseHasAnchor(lc) || seen[lc] {
			continue
		}
		if isNoisyPrimaryKeyword(phrase, pkg) || isAWSServiceName(phrase, awsSet) {
			continue
		}
		seen[lc] = true
		out = append(out, phrase)
	}
	return out
}

// splitPhraseChunks breaks text into runs of content words, splitting at
// connector/verb breaker words and at punctuation (commas, colons, etc.).
func splitPhraseChunks(text string) []string {
	var chunks []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			chunks = append(chunks, strings.Join(cur, " "))
			cur = nil
		}
	}
	for _, raw := range strings.Fields(collapse(text)) {
		// Punctuation other than an internal hyphen ends the current chunk.
		hadTrailingPunct := strings.ContainsAny(raw, ",:;!?()\"")
		w := strings.Trim(raw, ".,:;!?\"'()[]{}")
		if w == "" {
			flush()
			continue
		}
		if phraseBreakers[strings.ToLower(w)] {
			flush()
			continue
		}
		cur = append(cur, w)
		if hadTrailingPunct || strings.HasSuffix(raw, ".") {
			flush()
		}
	}
	flush()
	return chunks
}

// trimPhrase strips trailing low-value adjectives/adverbs from a chunk.
func trimPhrase(s string) string {
	words := strings.Fields(s)
	for len(words) > 0 && phraseTailTrim[strings.ToLower(words[len(words)-1])] {
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

// phraseHasAnchor reports whether any word (or hyphen part) of lc is a topic
// anchor.
func phraseHasAnchor(lc string) bool {
	for _, w := range strings.FieldsFunc(lc, func(r rune) bool { return r == ' ' || r == '-' }) {
		if topicAnchors[w] {
			return true
		}
	}
	return false
}
