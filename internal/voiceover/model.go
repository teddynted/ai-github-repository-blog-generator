// Package voiceover is the Narration engine (Milestone 5). It converts a
// generated Storyboard (Milestone 4) into a structured, synchronized voice-over
// script: the canonical input for future Text-to-Speech pipelines (Amazon Polly,
// ElevenLabs, OpenAI TTS, Azure Speech, Google Cloud TTS) and human narrators.
//
// It is responsible for NARRATION, not speech synthesis. It never regenerates
// the storyboard and never invents facts: every scene's narration, timing,
// diagram and code cues are drawn from the storyboard, which is itself grounded
// in the Release Context. It is deterministic where it matters — timestamps,
// speaking pace, pronunciation extraction, emphasis, pauses, transitions, sync
// cues, and content intelligence are all rule-based and independently testable.
// The shared releasegen.Model port is used only to polish narration wording,
// with a deterministic fallback (when Model is nil, narration is taken from the
// storyboard verbatim), so nothing is fabricated.
package voiceover

// SchemaVersion is the voice-over script document version (SemVer,
// additive-only) so future TTS milestones extend it without breaking.
const SchemaVersion = "1.0.0"

// VoiceOverScript is the complete narration plan for one storyboard.
type VoiceOverScript struct {
	SchemaVersion       string       `json:"schemaVersion"`
	Metadata            Metadata     `json:"metadata"`
	Voice               Voice        `json:"voice"`
	Scenes              []Scene      `json:"scenes"`
	ContentIntelligence Intelligence `json:"contentIntelligence"`
	Warnings            []string     `json:"warnings,omitempty"`
}

// Metadata identifies the source of the script.
type Metadata struct {
	Repository       string `json:"repository"`
	Release          string `json:"release"`
	SourceBlogTitle  string `json:"sourceBlogTitle"`
	GeneratedAt      string `json:"generatedAt"`
	SceneCount       int    `json:"sceneCount"`
	TotalDurationSec int    `json:"totalDurationSec"`
}

// Voice is the global, TTS-agnostic voice profile for the whole script. It is a
// direction for any downstream engine or human narrator — not a provider-
// specific voice ID, so the script works with every future TTS backend.
type Voice struct {
	Style            string `json:"style"`            // Professional, Educational, Confident, Clear, Technical, Conversational
	Persona          string `json:"persona"`          // "an experienced software engineer teaching another engineer"
	Tone             string `json:"tone"`             // warm, precise, encouraging
	DefaultPace      string `json:"defaultPace"`      // Slow | Conversational | Medium | Fast
	Language         string `json:"language"`         // BCP-47, e.g. en-US
	RecommendedVoice string `json:"recommendedVoice"` // provider-neutral hint (e.g. "Neural, en-US, warm, conversational")
	WordsPerMinute   int    `json:"wordsPerMinute"`   // baseline narration rate
}

// Scene is the narration plan for one storyboard scene.
type Scene struct {
	SceneNumber    int             `json:"sceneNumber"`
	Title          string          `json:"title"`
	Timestamp      Timestamp       `json:"timestamp"`
	Duration       Duration        `json:"duration"`
	Pace           string          `json:"pace"`
	VoiceDirection string          `json:"voiceDirection"`
	Emotion        string          `json:"emotion"`
	Energy         string          `json:"energy"` // low | medium | high
	OpeningCue     string          `json:"openingCue"`
	Narration      string          `json:"narration"`
	Pronunciation  []Pronunciation `json:"pronunciation,omitempty"`
	Emphasis       []string        `json:"emphasis,omitempty"`
	Pauses         []Pause         `json:"pauses,omitempty"`
	SyncCues       []SyncCue       `json:"syncCues,omitempty"`
	Transition     string          `json:"transition"`
	ClosingCue     string          `json:"closingCue"`
	WordCount      int             `json:"wordCount"`
}

// Timestamp places a scene on the video timeline. Both the human "MM:SS" label
// and the raw seconds are provided so TTS engines and editors can consume either.
type Timestamp struct {
	Start    string `json:"start"` // "MM:SS"
	End      string `json:"end"`   // "MM:SS"
	StartSec int    `json:"startSec"`
	EndSec   int    `json:"endSec"`
	Label    string `json:"label"` // "MM:SS–MM:SS"
}

// Duration is the timing plan for a scene's narration (seconds). It reconciles
// the storyboard's allocated time with the estimated time to speak the narration.
type Duration struct {
	AllocatedSec       int  `json:"allocatedSec"`       // from the storyboard scene
	EstimatedSpeechSec int  `json:"estimatedSpeechSec"` // time to speak the narration at the voice rate
	Fits               bool `json:"fits"`               // narration fits inside the allocated time (with tolerance)
}

// Pronunciation guides a narrator or TTS engine on a technical term.
type Pronunciation struct {
	Term     string `json:"term"`          // as it appears in the narration
	Phonetic string `json:"phonetic"`      // human-readable respelling, e.g. "kloud-for-MAY-shun"
	IPA      string `json:"ipa,omitempty"` // optional IPA
	SayAs    string `json:"sayAs"`         // spell-out | as-written | characters (SSML say-as interpret-as hint)
	Note     string `json:"note,omitempty"`
}

// Pause is a deliberate silence marker for pacing and emphasis.
type Pause struct {
	Type       string `json:"type"`                // short | medium | long
	Position   string `json:"position"`            // opening | before-diagram | before-demonstration | after-key-takeaway | closing
	DurationMs int    `json:"durationMs"`          // suggested silence (SSML <break time>)
	AfterText  string `json:"afterText,omitempty"` // narration fragment the pause follows, when applicable
	Note       string `json:"note,omitempty"`
}

// SyncCue aligns narration with a visual so the voice never describes something
// before it appears on screen. Cues are derived from the storyboard scene.
type SyncCue struct {
	Visual string `json:"visual"` // Diagram Reveal | Code Highlight | Camera Move | Overlay
	Cue    string `json:"cue"`    // when to speak relative to the visual
	Target string `json:"target,omitempty"`
}

// Intelligence is production metadata for the whole script.
type Intelligence struct {
	TotalWords             int      `json:"totalWords"`
	EstimatedSpeakingSec   int      `json:"estimatedSpeakingSec"`
	EstimatedSpeakingTime  string   `json:"estimatedSpeakingTime"` // "M:SS"
	AverageWordsPerMinute  int      `json:"averageWordsPerMinute"`
	ReadingDifficulty      string   `json:"readingDifficulty"`      // easy | moderate | advanced
	TechnicalDensity       string   `json:"technicalDensity"`       // low | medium | high
	EstimatedRecordingTime string   `json:"estimatedRecordingTime"` // "M:SS" incl. retakes
	VoiceStyle             string   `json:"voiceStyle"`
	RecommendedTTSVoice    string   `json:"recommendedTTSVoice"`
	RecommendedLanguage    string   `json:"recommendedLanguage"`
	UniquePronunciations   int      `json:"uniquePronunciations"`
	ProductionNotes        []string `json:"productionNotes,omitempty"`
}
