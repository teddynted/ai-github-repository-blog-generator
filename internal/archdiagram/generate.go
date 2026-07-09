package archdiagram

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
)

// Result is the complete architecture-diagram output for a repository, ready to
// embed into the blog. Markdown() renders it as a single blog section.
type Result struct {
	RepoFullName string
	Summary      string
	Logical      []string  // detected logical components
	Services     []Service // AWS service mapping (High/Medium first)
	Diagrams     []Diagram
	Detection    Detection
}

// Generate performs detection and builds every applicable diagram. It is
// deterministic and never blocks on external calls, so it is safe to run inline
// in the pipeline.
func Generate(snap processing.Snapshot) Result {
	det := Detect(snap)
	res := Result{
		RepoFullName: snap.RepoFullName,
		Summary:      summarize(snap, det),
		Logical:      logicalComponents(det),
		Services:     orderForReport(det.Services),
		Diagrams:     det.buildDiagrams(),
		Detection:    det,
	}
	return res
}

// Markdown renders the result as a self-contained blog section that satisfies
// the required output structure (summary → components → mapping → diagrams →
// confidence report → evidence report).
func (r Result) Markdown() string {
	var b strings.Builder
	b.WriteString("# AWS Architecture\n\n")
	b.WriteString(r.Summary)
	b.WriteString("\n\n")

	b.WriteString("## Detected Components\n\n")
	if len(r.Logical) == 0 {
		b.WriteString("_No distinct logical components were detected with sufficient confidence._\n\n")
	} else {
		for _, c := range r.Logical {
			fmt.Fprintf(&b, "- %s\n", c)
		}
		b.WriteByte('\n')
	}

	b.WriteString("## AWS Service Mapping\n\n")
	b.WriteString("| Logical component | AWS service | Confidence | Evidence |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, s := range r.Services {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			s.Logical, s.Name, s.Confidence, firstEvidence(s))
	}
	b.WriteByte('\n')

	for _, dg := range r.Diagrams {
		fmt.Fprintf(&b, "## %s\n\n", dg.Title)
		fmt.Fprintf(&b, "%s\n\n", dg.Description)
		b.WriteString("```mermaid\n")
		b.WriteString(dg.Mermaid)
		b.WriteString("\n```\n\n")
		fmt.Fprintf(&b, "**Purpose:** %s\n\n", dg.Purpose)
	}

	b.WriteString(r.confidenceReport())
	b.WriteString(r.evidenceReport())
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func (r Result) confidenceReport() string {
	var b strings.Builder
	b.WriteString("## Confidence Report\n\n")
	b.WriteString("Only High and Medium confidence services appear in the primary AWS diagram; Low confidence services are listed here for transparency but omitted from the architecture to avoid speculation.\n\n")
	for _, level := range []Confidence{High, Medium, Low} {
		names := servicesAt(r.Services, level)
		if len(names) == 0 {
			continue
		}
		fmt.Fprintf(&b, "- **%s:** %s\n", level, strings.Join(names, ", "))
	}
	b.WriteByte('\n')
	return b.String()
}

func (r Result) evidenceReport() string {
	var b strings.Builder
	b.WriteString("## Repository Evidence\n\n")
	b.WriteString("Every service above is grounded in repository evidence or explicit deployment context:\n\n")
	for _, s := range r.Services {
		fmt.Fprintf(&b, "- **%s** — %s\n", s.Name, strings.Join(s.Evidence, "; "))
	}
	b.WriteByte('\n')
	return b.String()
}

// --- pipeline integration ------------------------------------------------

// Diagrammer adapts Generate to the pipeline's optional diagram seam. It returns
// an architecture-diagram content asset, and false when nothing worth drawing
// was detected (a bare repository with no evidence).
type Diagrammer struct{}

// Diagram implements pipeline.Diagrammer.
func (Diagrammer) Diagram(_ context.Context, snap processing.Snapshot) (generation.Content, bool, error) {
	res := Generate(snap)
	if len(res.Diagrams) == 0 {
		return generation.Content{}, false, nil
	}
	return generation.Content{
		Kind:     generation.KindArchitectureDiagram,
		Markdown: res.Markdown(),
	}, true, nil
}

// --- report helpers ------------------------------------------------------

func summarize(snap processing.Snapshot, d Detection) string {
	n := 0
	for _, s := range d.Services {
		if s.Confidence != Low && s.ID != "github" {
			n++
		}
	}
	langs := joinTop(d.Languages, 3)
	repo := snap.RepoFullName
	if repo == "" {
		repo = "the repository"
	}
	return fmt.Sprintf(
		"This architecture is derived directly from %s (%s). It maps the repository onto %d AWS service(s) supported by concrete evidence, using an Amazon EC2 (Spot) compute substrate and self-hosted OpenClaw for any LLM inference. Services that could not be confirmed from the repository were deliberately omitted.",
		repo, langs, n)
}

func logicalComponents(d Detection) []string {
	set := map[string]bool{}
	var out []string
	addRole := func(role string) {
		if role == "" || set[role] {
			return
		}
		set[role] = true
		out = append(out, role)
	}
	addRole("Source repository (GitHub)")
	addRole(d.Compute.Logical)
	if d.HasAPI {
		addRole("API layer")
	}
	for _, s := range d.Services {
		if s.Confidence != Low && s.ID != "github" && s.ID != d.Compute.ID {
			addRole(s.Logical)
		}
	}
	if d.HasCICD {
		addRole("CI/CD pipeline")
	}
	if d.HasAI {
		addRole("LLM inference")
	}
	return out
}

// orderForReport sorts services by confidence (High→Low) then name, and drops
// nothing — the evidence report is meant to be complete.
func orderForReport(services []Service) []Service {
	out := append([]Service(nil), services...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence.rank() != out[j].Confidence.rank() {
			return out[i].Confidence.rank() > out[j].Confidence.rank()
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func servicesAt(services []Service, level Confidence) []string {
	var names []string
	for _, s := range services {
		if s.Confidence == level {
			names = append(names, s.Name)
		}
	}
	sort.Strings(names)
	return names
}

func firstEvidence(s Service) string {
	if len(s.Evidence) == 0 {
		return "—"
	}
	return s.Evidence[0]
}
