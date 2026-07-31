package voiceover

import (
	"regexp"
	"strconv"
	"strings"
)

// versionToken matches a dotted numeric token — a semantic version (1.0.0) or a
// decimal (3.5). Neural TTS engines read these unpredictably ("one dot zero dot
// zero", or worse), so they are spelled out for speech.
var versionToken = regexp.MustCompile(`\d+(?:\.\d+)+`)

// normalizeForSpeech rewrites TTS-hostile tokens in narration into their natural
// spoken form. It currently spells dotted numbers: "1.0.0" becomes
// "one point zero point zero", "1.0.1" becomes "one point zero point one". Only
// components that are plain integers in [0,99] are spelled; anything outside that
// (or a token that fails to parse) is left untouched, so unusual text is never
// mangled. Deterministic and provider-neutral.
func normalizeForSpeech(s string) string {
	return versionToken.ReplaceAllStringFunc(s, func(tok string) string {
		parts := strings.Split(tok, ".")
		words := make([]string, 0, len(parts))
		for _, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				return tok // non-integer component: leave the whole token as-is
			}
			w, ok := numberWord(n)
			if !ok {
				return tok // out of range: leave as-is rather than guess
			}
			words = append(words, w)
		}
		return strings.Join(words, " point ")
	})
}

var (
	onesWords = []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	teenWords = []string{"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tensWords = []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
)

// numberWord spells an integer in [0,99] as English words ("zero", "twenty-one").
// Returns false outside that range so callers can leave the token untouched.
func numberWord(n int) (string, bool) {
	switch {
	case n < 0 || n > 99:
		return "", false
	case n < 10:
		return onesWords[n], true
	case n < 20:
		return teenWords[n-10], true
	default:
		w := tensWords[n/10]
		if n%10 != 0 {
			w += "-" + onesWords[n%10]
		}
		return w, true
	}
}
