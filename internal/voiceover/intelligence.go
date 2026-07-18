package voiceover

import "fmt"

// recordingOverhead approximates human retakes/edits: recording time is ~1.8×
// the clean speaking time.
const recordingOverhead = 1.8

// planIntelligence aggregates whole-script production metadata from the finished
// scenes. All values are derived deterministically from the narration.
func planIntelligence(scenes []Scene, voice Voice, uniqueTerms int) Intelligence {
	words, speakSec := 0, 0
	for _, s := range scenes {
		words += s.WordCount
		speakSec += s.Duration.EstimatedSpeechSec
	}
	density := technicalDensity(uniqueTerms, words)
	return Intelligence{
		TotalWords:             words,
		EstimatedSpeakingSec:   speakSec,
		EstimatedSpeakingTime:  mmss(speakSec),
		AverageWordsPerMinute:  voice.WordsPerMinute,
		ReadingDifficulty:      readingDifficulty(density),
		TechnicalDensity:       density,
		EstimatedRecordingTime: mmss(int(float64(speakSec) * recordingOverhead)),
		VoiceStyle:             voice.Style,
		RecommendedTTSVoice:    voice.RecommendedVoice,
		RecommendedLanguage:    voice.Language,
		UniquePronunciations:   uniqueTerms,
		ProductionNotes:        productionNotes(density, uniqueTerms),
	}
}

// technicalDensity classifies how term-heavy the script is (unique technical
// terms per 100 words).
func technicalDensity(uniqueTerms, words int) string {
	if words == 0 {
		return "low"
	}
	per100 := float64(uniqueTerms) / float64(words) * 100
	switch {
	case per100 >= 6:
		return "high"
	case per100 >= 2.5:
		return "medium"
	default:
		return "low"
	}
}

func readingDifficulty(density string) string {
	switch density {
	case "high":
		return "advanced"
	case "medium":
		return "moderate"
	default:
		return "easy"
	}
}

func productionNotes(density string, uniqueTerms int) []string {
	notes := []string{
		"Narration is TTS-agnostic: pronunciation entries carry SSML say-as hints; pauses carry break durations.",
	}
	if uniqueTerms > 0 {
		notes = append(notes, fmt.Sprintf("Load the %d pronunciation entries into your TTS lexicon (or brief the narrator) before recording.", uniqueTerms))
	}
	if density == "high" {
		notes = append(notes, "High technical density — favour a slower default pace and generous pauses around diagrams and code.")
	}
	return notes
}
