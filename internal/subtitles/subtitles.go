// Package subtitles builds deterministic caption files (SubRip .srt and Advanced
// SubStation Alpha .ass) from timed narration words — NOT from speech-to-text.
//
// The words and their timings come from Amazon Polly speech marks for the exact
// narration the TTS speaks (which is itself derived from blog.md via the
// storyboard). So a term defined once in blog.md propagates unchanged into the
// captions: there is no transcription step and therefore no phonetic
// hallucination (never "claw"→"cloud", "Ollama"→"Obama", "n8n"→"Nathan").
//
// These are SIDECAR files shipped alongside the MP4 (for platform CC / upload /
// accessibility); the renderer does not burn them into the frame.
package subtitles

import (
	"fmt"
	"strings"
)

// Mobile-first caption constraints (vertical short-form video).
const (
	MaxLineChars = 42 // characters per line
	MaxLines     = 2  // lines per cue
)

// TimedWord is one spoken word with its start/end time in the FINAL video (ms),
// as reported by Polly speech marks plus the scene's offset in the concatenation.
type TimedWord struct {
	Word    string
	StartMs int
	EndMs   int
}

// Cue is one on-screen subtitle: 1–2 lines shown over [StartMs, EndMs].
type Cue struct {
	StartMs int
	EndMs   int
	Lines   []string
}

// Wrap groups timed words into cues: at most MaxLines lines of at most
// MaxLineChars characters, breaking at sentence boundaries, with each cue timed
// directly from its words' marks. atomic lists multi-word protected phrases
// (e.g. "Amazon Bedrock", "AWS Lambda") that must never break across a line —
// they are packed as one unit. Deterministic: identical input → identical output.
func Wrap(words []TimedWord, atomic []string) []Cue {
	tokens := mergeAtomic(words, atomic)
	var cues []Cue
	var lines []string
	cur := ""
	var startMs, endMs int
	haveCue := false

	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
		if len(lines) > 0 {
			cues = append(cues, Cue{StartMs: startMs, EndMs: endMs, Lines: lines})
		}
		lines = nil
		haveCue = false
	}

	for _, tk := range tokens {
		if !haveCue {
			startMs = tk.StartMs
			haveCue = true
		}
		endMs = tk.EndMs
		// Would this token overflow the current line?
		candidate := tk.Word
		if cur != "" {
			candidate = cur + " " + tk.Word
		}
		if len(candidate) > MaxLineChars && cur != "" {
			lines = append(lines, cur)
			cur = tk.Word
			if len(lines) >= MaxLines {
				// Cue full — close it (the just-started word carries into the next).
				pending := cur
				ps, pe := tk.StartMs, tk.EndMs
				cur = ""
				flush()
				cur = pending
				startMs, endMs, haveCue = ps, pe, true
			}
		} else {
			cur = candidate
		}
		// Break the cue at a sentence boundary so captions align to spoken sentences.
		if endsSentence(tk.Word) {
			flush()
		}
	}
	flush()
	return cues
}

// mergeAtomic collapses consecutive words that form a protected multi-word phrase
// into a single token (with combined timing) so the phrase can never be split
// across a line break. Matching is case-sensitive on the exact phrase words.
func mergeAtomic(words []TimedWord, atomic []string) []TimedWord {
	seqs := make([][]string, 0, len(atomic))
	for _, a := range atomic {
		if f := strings.Fields(a); len(f) > 1 {
			seqs = append(seqs, f)
		}
	}
	var out []TimedWord
	for i := 0; i < len(words); {
		matched := false
		for _, seq := range seqs {
			if matchSeq(words, i, seq) {
				merged := TimedWord{
					Word:    strings.Join(seq, " "),
					StartMs: words[i].StartMs,
					EndMs:   words[i+len(seq)-1].EndMs,
				}
				out = append(out, merged)
				i += len(seq)
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, words[i])
			i++
		}
	}
	return out
}

// matchSeq reports whether words[i:] begins with seq (comparing the alphanumeric
// core of each word, so trailing punctuation on the last word still matches).
func matchSeq(words []TimedWord, i int, seq []string) bool {
	if i+len(seq) > len(words) {
		return false
	}
	for k, s := range seq {
		if core(words[i+k].Word) != s {
			return false
		}
	}
	return true
}

// core strips leading/trailing punctuation for phrase matching (keeps the token
// itself intact in output).
func core(w string) string { return strings.Trim(w, ".,;:!?\"'()") }

func endsSentence(w string) bool {
	w = strings.TrimRight(w, "\"')")
	return strings.HasSuffix(w, ".") || strings.HasSuffix(w, "!") || strings.HasSuffix(w, "?")
}

// SRT renders cues as valid UTF-8 SubRip.
func SRT(cues []Cue) string {
	var b strings.Builder
	for i, c := range cues {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, srtTime(c.StartMs), srtTime(c.EndMs), strings.Join(c.Lines, "\n"))
	}
	return b.String()
}

// ASS renders cues as Advanced SubStation Alpha with a mobile-first vertical
// style: white text, black outline, semi-transparent box, bottom-centre, with
// margins that clear the TikTok/Shorts UI. w×h set the caption reference frame.
func ASS(cues []Cue, w, h int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nWrapStyle: 2\n\n", w, h)
	b.WriteString("[V4+ Styles]\n")
	b.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	b.WriteString("Style: Default,Inter,54,&H00FFFFFF,&H000000FF,&H00000000,&H80000000,-1,0,0,0,100,100,0,0,1,3,0,2,80,80,120,1\n\n")
	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for _, c := range cues {
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n", assTime(c.StartMs), assTime(c.EndMs), strings.Join(c.Lines, `\N`))
	}
	return b.String()
}

func srtTime(ms int) string {
	h, m, s, milli := clock(ms)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, milli)
}

func assTime(ms int) string {
	h, m, s, milli := clock(ms)
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, milli/10)
}

func clock(ms int) (h, m, s, milli int) {
	if ms < 0 {
		ms = 0
	}
	milli = ms % 1000
	total := ms / 1000
	s = total % 60
	m = (total / 60) % 60
	h = total / 3600
	return
}
