package governance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// ModelReviewer is an AIReviewer backed by the shared releasegen.Model port, so
// it works with Amazon Bedrock or Anthropic (Claude), or any future provider. It is
// grounded: the prompt supplies the grounding terms and forbids inventing facts.
type ModelReviewer struct {
	Model releasegen.Model
	Name  string // reviewer label, e.g. "bedrock" or "anthropic"
}

// NewModelReviewer wraps a Model as an AIReviewer.
func NewModelReviewer(m releasegen.Model, name string) *ModelReviewer {
	if name == "" {
		name = "model"
	}
	return &ModelReviewer{Model: m, Name: name}
}

// Review asks the model for qualitative notes and parses them. On any error or
// unparseable output it returns an error so the engine falls back to
// deterministic notes — it never blocks or fabricates.
func (r *ModelReviewer) Review(ctx context.Context, c Content) (AINotes, error) {
	if r.Model == nil {
		return AINotes{}, ErrReviewFailed
	}
	out, err := r.Model.Generate(ctx, reviewPrompt(c))
	if err != nil {
		return AINotes{}, err
	}
	notes, err := parseNotes(out)
	if err != nil {
		return AINotes{}, err
	}
	notes.Reviewer = r.Name
	return notes, nil
}

func reviewPrompt(c Content) string {
	terms := strings.Join(topStrings(c.Grounding.Terms, 30), ", ")
	return fmt.Sprintf(
		"You are a senior technical reviewer. Review the %s below for technical correctness, factual accuracy, "+
			"clarity, readability, educational value, and SEO. It MUST only reference these grounded facts "+
			"(from the release context): [%s]. Flag anything not supported by them as a hallucination.\n\n"+
			"Respond ONLY with minified JSON of the form: "+
			"{\"summary\":\"...\",\"strengths\":[\"...\"],\"weaknesses\":[\"...\"],\"suggestions\":[\"...\"],\"confidence\":0-100}. "+
			"Do not invent facts. Do not add fields.\n\nTITLE: %s\n\nCONTENT:\n%s",
		c.Type, terms, c.Title, truncate(c.Body, 6000))
}

// parseNotes extracts the JSON review object from the model output (tolerating
// surrounding prose or code fences).
func parseNotes(out string) (AINotes, error) {
	s := strings.TrimSpace(out)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return AINotes{}, ErrReviewFailed
	}
	var n AINotes
	if err := json.Unmarshal([]byte(s[start:end+1]), &n); err != nil {
		return AINotes{}, ErrReviewFailed
	}
	n.Confidence = clampInt(n.Confidence, 0, 100)
	return n, nil
}
