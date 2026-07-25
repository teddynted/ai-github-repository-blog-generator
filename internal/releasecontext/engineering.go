package releasecontext

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EngineeringContext is the structured engineering analysis of a release — the
// single source of truth that sits BETWEEN the deterministic analysis models
// (Ollama) and the technical-writer model (Claude).
//
// The pipeline is deliberately split by responsibility:
//
//   - Stage 1 (repository analysis) produces the factual ReleaseContext.
//   - Stage 2 (Ollama) reads those facts and EXTRACTS this structured JSON —
//     the "why" behind the release: decisions, trade-offs, service choices,
//     security posture, scalability, cost, and what becomes possible next. It
//     writes no prose.
//   - Stage 3 (Claude) consumes this JSON (never the raw repo dump) to write
//     publication-ready narrative content.
//
// Keeping the contract typed and JSON-serialisable means the two model stages
// are decoupled: either model can be swapped (Provider Abstraction Layer, MCP)
// without changing the other, and the extraction can be inspected, cached, and
// reviewed on its own.
type EngineeringContext struct {
	Release              ReleaseRef            `json:"release"`
	Problem              string                `json:"problem"`
	EngineeringDecisions []EngineeringDecision `json:"engineering_decisions,omitempty"`
	ArchitecturalDrivers []string              `json:"architectural_drivers,omitempty"`
	Tradeoffs            []Tradeoff            `json:"tradeoffs,omitempty"`
	AWSServices          []AWSServiceChoice    `json:"aws_services,omitempty"`
	Security             []string              `json:"security,omitempty"`
	Scalability          Scalability           `json:"scalability"`
	CostOptimizations    []string              `json:"cost_optimizations,omitempty"`
	FutureMilestones     []string              `json:"future_milestones,omitempty"`
	ImplementationNotes  []string              `json:"implementation_notes,omitempty"`
	LessonsLearned       []string              `json:"lessons_learned,omitempty"`
}

// ReleaseRef is the minimal release identity carried inside the engineering
// analysis, so the JSON is self-describing when inspected in isolation.
type ReleaseRef struct {
	Version string `json:"version"`
	Name    string `json:"name,omitempty"`
}

// EngineeringDecision captures a change and the reasoning behind it — the
// difference between a release note ("added X") and an engineering narrative
// ("chose X because Y, over Z").
type EngineeringDecision struct {
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
}

// Tradeoff records an alternative that was weighed, with its pros and cons — the
// material that turns a summary into a story about engineering judgement.
type Tradeoff struct {
	Choice        string   `json:"choice"`
	Alternatives  []string `json:"alternatives,omitempty"`
	Advantages    []string `json:"advantages,omitempty"`
	Disadvantages []string `json:"disadvantages,omitempty"`
}

// AWSServiceChoice explains a service introduced by the release and why it was
// chosen — grounded strictly in evidence from the repository.
type AWSServiceChoice struct {
	Name      string `json:"name"`
	Purpose   string `json:"purpose"`
	Rationale string `json:"rationale,omitempty"`
}

// Scalability describes the current and future scaling strategy.
type Scalability struct {
	Current string `json:"current,omitempty"`
	Future  string `json:"future,omitempty"`
}

// IsZero reports whether the analysis is effectively empty (no problem statement
// and no extracted lists), so callers can treat a failed/degenerate extraction
// as absent and fall back gracefully.
func (e *EngineeringContext) IsZero() bool {
	if e == nil {
		return true
	}
	return strings.TrimSpace(e.Problem) == "" &&
		len(e.EngineeringDecisions) == 0 &&
		len(e.Tradeoffs) == 0 &&
		len(e.AWSServices) == 0 &&
		len(e.Security) == 0 &&
		len(e.CostOptimizations) == 0 &&
		len(e.FutureMilestones) == 0 &&
		len(e.ImplementationNotes) == 0
}

// JSON renders the analysis as indented JSON — the exact payload handed to the
// technical-writer stage.
func (e *EngineeringContext) JSON() string {
	if e == nil {
		return "{}"
	}
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

// GroundingBlock renders the analysis as a compact, human-readable block for
// inclusion in a generation prompt's grounding section. It is deterministic
// (stable ordering) so prompts — and therefore outputs — are reproducible.
func (e *EngineeringContext) GroundingBlock() string {
	if e.IsZero() {
		return ""
	}
	var b strings.Builder
	b.WriteString("=== ENGINEERING ANALYSIS (structured; the reasoning behind this release) ===\n")
	if v := strings.TrimSpace(e.Problem); v != "" {
		fmt.Fprintf(&b, "Problem solved: %s\n", v)
	}
	writeDecisions(&b, e.EngineeringDecisions)
	writeList(&b, "Architectural drivers", e.ArchitecturalDrivers)
	writeTradeoffs(&b, e.Tradeoffs)
	writeServices(&b, e.AWSServices)
	writeList(&b, "Security", e.Security)
	if c := strings.TrimSpace(e.Scalability.Current); c != "" {
		fmt.Fprintf(&b, "Scalability (current): %s\n", c)
	}
	if f := strings.TrimSpace(e.Scalability.Future); f != "" {
		fmt.Fprintf(&b, "Scalability (future): %s\n", f)
	}
	writeList(&b, "Cost optimizations", e.CostOptimizations)
	writeList(&b, "Lessons learned", e.LessonsLearned)
	writeList(&b, "Future milestones unlocked", e.FutureMilestones)
	writeList(&b, "Implementation notes", e.ImplementationNotes)
	b.WriteString("=== END ENGINEERING ANALYSIS ===\n")
	return b.String()
}

func writeDecisions(b *strings.Builder, ds []EngineeringDecision) {
	if len(ds) == 0 {
		return
	}
	b.WriteString("Engineering decisions:\n")
	for _, d := range ds {
		if strings.TrimSpace(d.Decision) == "" {
			continue
		}
		if r := strings.TrimSpace(d.Rationale); r != "" {
			fmt.Fprintf(b, "  - %s — because %s\n", d.Decision, r)
		} else {
			fmt.Fprintf(b, "  - %s\n", d.Decision)
		}
	}
}

func writeTradeoffs(b *strings.Builder, ts []Tradeoff) {
	if len(ts) == 0 {
		return
	}
	b.WriteString("Trade-offs:\n")
	for _, t := range ts {
		if strings.TrimSpace(t.Choice) == "" {
			continue
		}
		fmt.Fprintf(b, "  - Chose %s\n", t.Choice)
		if len(t.Alternatives) > 0 {
			fmt.Fprintf(b, "      over: %s\n", strings.Join(t.Alternatives, ", "))
		}
		if len(t.Advantages) > 0 {
			fmt.Fprintf(b, "      pros: %s\n", strings.Join(t.Advantages, "; "))
		}
		if len(t.Disadvantages) > 0 {
			fmt.Fprintf(b, "      cons: %s\n", strings.Join(t.Disadvantages, "; "))
		}
	}
}

func writeServices(b *strings.Builder, ss []AWSServiceChoice) {
	if len(ss) == 0 {
		return
	}
	b.WriteString("AWS services:\n")
	for _, s := range ss {
		if strings.TrimSpace(s.Name) == "" {
			continue
		}
		line := "  - " + s.Name
		if p := strings.TrimSpace(s.Purpose); p != "" {
			line += ": " + p
		}
		if r := strings.TrimSpace(s.Rationale); r != "" {
			line += " (why: " + r + ")"
		}
		b.WriteString(line + "\n")
	}
}

func writeList(b *strings.Builder, label string, items []string) {
	nonEmpty := make([]string, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it) != "" {
			nonEmpty = append(nonEmpty, strings.TrimSpace(it))
		}
	}
	if len(nonEmpty) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", label)
	for _, it := range nonEmpty {
		fmt.Fprintf(b, "  - %s\n", it)
	}
}
