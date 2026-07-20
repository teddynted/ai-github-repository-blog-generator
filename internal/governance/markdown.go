package governance

import (
	"fmt"
	"strings"
)

// Markdown renders a workflow state as a human-readable governance report: the
// current status, the latest quality report, quality gates, publication
// readiness, revisions, and the full audit trail.
func (s *WorkflowState) Markdown() string {
	var b strings.Builder
	c := s.Content

	fmt.Fprintf(&b, "# Governance Report: %s\n\n", firstNonEmpty(c.Title, c.ID))
	fmt.Fprintf(&b, "_%s · %s · status **%s**_\n\n", c.Type, firstNonEmpty(c.Grounding.Release, "—"), s.Status)

	if r := s.LatestReport; r != nil {
		b.WriteString("## Latest Quality Review\n\n")
		fmt.Fprintf(&b, "- **Reviewer:** %s · **Decision:** %s · **Confidence:** %d/100\n", r.Reviewer, r.Decision, r.Confidence)
		fmt.Fprintf(&b, "- **Summary:** %s\n\n", r.Summary)
		writeScores(&b, r.Scores)
		writeValidation(&b, r.Validation)
		writeGrounding(&b, r.Grounding)
		writeList(&b, "Strengths", r.Strengths)
		writeList(&b, "Weaknesses", r.Weaknesses)
		writeList(&b, "Suggestions", r.Suggestions)
	}

	if s.Readiness != nil {
		b.WriteString("## Publication Readiness\n\n")
		fmt.Fprintf(&b, "**Ready:** %v\n\n", s.Readiness.Ready)
		for _, ck := range s.Readiness.Checks {
			fmt.Fprintf(&b, "- %s %s%s\n", checkMark(ck.Passed), ck.Name, detailSuffix(ck.Detail))
		}
		b.WriteString("\n")
	}

	if len(s.Revisions) > 0 {
		b.WriteString("## Revisions\n\n")
		for _, rev := range s.Revisions {
			fmt.Fprintf(&b, "- **#%d** (%s, %d chars changed)\n", rev.Number, rev.CreatedAt.Format("2006-01-02 15:04"), rev.ChangedChars)
			for _, f := range topStrings(rev.Feedback, 4) {
				fmt.Fprintf(&b, "  - feedback: %s\n", collapse(f))
			}
		}
		b.WriteString("\n")
	}

	if len(s.Approvals) > 0 {
		b.WriteString("## Approvals\n\n")
		for _, a := range s.Approvals {
			fmt.Fprintf(&b, "- %s **%s** (%s)%s\n", checkMark(a.Approved), a.Reviewer.Name, a.Reviewer.Role, detailSuffix(a.Comment))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Audit Trail\n\n")
	for _, e := range s.Audit {
		fmt.Fprintf(&b, "- `%s` **%s** by %s", e.Timestamp.Format("2006-01-02 15:04:05"), e.Action, e.Actor)
		if e.FromStatus != "" && e.FromStatus != e.ToStatus {
			fmt.Fprintf(&b, " (%s → %s)", e.FromStatus, e.ToStatus)
		}
		if e.Detail != "" {
			fmt.Fprintf(&b, " — %s", e.Detail)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// GatesMarkdown renders the pre-approval quality gates for a state.
func GatesMarkdown(gates []QualityGate) string {
	var b strings.Builder
	b.WriteString("## Quality Gates\n\n")
	for _, g := range gates {
		fmt.Fprintf(&b, "- %s %s%s\n", checkMark(g.Passed), g.Name, detailSuffix(g.Detail))
	}
	return b.String()
}

func writeScores(b *strings.Builder, s Scores) {
	b.WriteString("**Scores:**\n\n")
	fmt.Fprintf(b, "| Dimension | Score |\n|---|---|\n")
	rows := [][2]string{
		{"Overall", itoa(s.Overall)},
		{"Technical Accuracy", itoa(s.TechnicalAccuracy)},
		{"Completeness", itoa(s.Completeness)},
		{"Readability", itoa(s.Readability)},
		{"Grammar", itoa(s.Grammar)},
		{"Architecture", itoa(s.Architecture)},
		{"Consistency", itoa(s.Consistency)},
		{"Educational Value", itoa(s.EducationalValue)},
		{"SEO", itoa(s.SEO)},
	}
	for _, r := range rows {
		fmt.Fprintf(b, "| %s | %s |\n", r[0], r[1])
	}
	b.WriteString("\n")
}

func writeValidation(b *strings.Builder, v ValidationReport) {
	fmt.Fprintf(b, "**Validation:** %d error(s), %d warning(s)%s\n\n", v.Errors, v.Warnings, passSuffix(v.Passed))
	for _, i := range v.Issues {
		fmt.Fprintf(b, "- [%s] %s%s\n", i.Severity, i.Message, fieldSuffix(i.Field))
	}
	if len(v.Issues) > 0 {
		b.WriteString("\n")
	}
}

func writeGrounding(b *strings.Builder, g GroundingResult) {
	fmt.Fprintf(b, "**Grounding:** %v — %d/%d claims grounded\n\n", g.Verified, g.GroundedClaims, g.CheckedClaims)
	for _, c := range g.UnverifiedClaims {
		fmt.Fprintf(b, "- ⚠ ungrounded: %s\n", truncate(c, 120))
	}
	if len(g.UnverifiedClaims) > 0 {
		b.WriteString("\n")
	}
}

func writeList(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "**%s:**\n", label)
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", collapse(it))
	}
	b.WriteString("\n")
}

func checkMark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

func detailSuffix(d string) string {
	if d == "" {
		return ""
	}
	return " — " + d
}

func fieldSuffix(f string) string {
	if f == "" {
		return ""
	}
	return " (" + f + ")"
}

func passSuffix(ok bool) string {
	if ok {
		return " — passed"
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
