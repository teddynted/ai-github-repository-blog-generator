package releasecontext

import (
	"path"
	"sort"
	"strings"
)

// analyzeDocumentation reads the README and docs/ markdown into structured
// documentation insights.
func analyzeDocumentation(files []RawFile) Documentation {
	var doc Documentation
	var readme string

	for _, f := range files {
		if !isMarkdownPath(f.Path) || f.Content == "" {
			continue
		}
		ref := DocumentRef{
			Path:     f.Path,
			Title:    firstHeading(f.Content),
			Kind:     docKind(f.Path),
			Headings: topN(headings(f.Content), 25),
		}
		doc.Documents = append(doc.Documents, ref)
		if ref.Kind == "readme" && strings.EqualFold(path.Base(f.Path), "readme.md") && readme == "" {
			readme = f.Content
		}
	}
	sort.Slice(doc.Documents, func(i, j int) bool { return doc.Documents[i].Path < doc.Documents[j].Path })

	if readme != "" {
		doc.Purpose = firstSentence(firstParagraph(readme))
	}
	corpus := docCorpus(files)
	doc.ArchitecturePatterns = detectPatterns(corpus)
	doc.DeploymentStrategy = detectDeployment(corpus)
	doc.DevelopmentWorkflow = detectWorkflow(corpus)
	doc.TechnicalGoals = detectHeadingBullets(files, "goal", "objective")
	doc.DesignDecisions = detectHeadingBullets(files, "design decision", "decision", "principle")
	if doc.BusinessProblem == "" {
		doc.BusinessProblem = detectBusinessProblem(corpus)
	}
	return doc
}

func docKind(p string) string {
	lp := strings.ToLower(p)
	base := strings.ToLower(path.Base(p))
	switch {
	case base == "readme.md":
		return "readme"
	case strings.Contains(lp, "adr") || strings.Contains(lp, "decision"):
		return "adr"
	case strings.Contains(lp, "architecture"):
		return "architecture"
	case strings.Contains(lp, "deploy"):
		return "deployment"
	case strings.HasPrefix(lp, "docs/"):
		return "guide"
	default:
		return "other"
	}
}

func firstHeading(content string) string {
	for _, ln := range strings.Split(content, "\n") {
		if strings.HasPrefix(ln, "# ") {
			return strings.TrimSpace(stripMarkdown(strings.TrimPrefix(ln, "# ")))
		}
	}
	return ""
}

func headings(content string) []string {
	var hs []string
	for _, ln := range strings.Split(content, "\n") {
		if strings.HasPrefix(ln, "## ") {
			hs = append(hs, strings.TrimSpace(stripMarkdown(strings.TrimPrefix(ln, "## "))))
		} else if strings.HasPrefix(ln, "### ") {
			hs = append(hs, strings.TrimSpace(stripMarkdown(strings.TrimPrefix(ln, "### "))))
		}
	}
	return hs
}

// firstParagraph returns the first non-heading, non-empty prose block.
func firstParagraph(content string) string {
	var para []string
	for _, ln := range strings.Split(content, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "<") ||
			strings.HasPrefix(t, "[!") || strings.HasPrefix(t, "!") || strings.HasPrefix(t, "```") {
			continue
		}
		para = append(para, t)
	}
	return strings.Join(para, " ")
}

func docCorpus(files []RawFile) string {
	var b strings.Builder
	for _, f := range files {
		if isMarkdownPath(f.Path) {
			b.WriteString(strings.ToLower(f.Content))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func detectPatterns(corpus string) []string {
	patterns := []struct{ needle, name string }{
		{"event-driven", "Event-driven architecture"},
		{"serverless", "Serverless"},
		{"clean architecture", "Clean Architecture"},
		{"ports and adapters", "Ports & Adapters"},
		{"hexagonal", "Hexagonal architecture"},
		{"microservice", "Microservices"},
		{"infrastructure as code", "Infrastructure as Code"},
		{"least-privilege", "Least-privilege security"},
		{"dependency injection", "Dependency Injection"},
	}
	var out []string
	for _, p := range patterns {
		if strings.Contains(corpus, p.needle) {
			out = append(out, p.name)
		}
	}
	return out
}

func detectDeployment(corpus string) string {
	switch {
	case strings.Contains(corpus, "cloudformation") && strings.Contains(corpus, "github actions"):
		return "Infrastructure as Code with AWS CloudFormation, deployed via GitHub Actions (OIDC)."
	case strings.Contains(corpus, "cloudformation"):
		return "Infrastructure as Code with AWS CloudFormation."
	case strings.Contains(corpus, "terraform"):
		return "Infrastructure as Code with Terraform."
	default:
		return ""
	}
}

func detectWorkflow(corpus string) string {
	if strings.Contains(corpus, "conventional commit") {
		return "Trunk-based development with Conventional Commits and automated release management."
	}
	if strings.Contains(corpus, "pull request") {
		return "Pull-request based workflow with CI checks before merge."
	}
	return ""
}

func detectBusinessProblem(corpus string) string {
	if strings.Contains(corpus, "opt in") || strings.Contains(corpus, "opt-in") {
		return "Automating high-quality technical content generation from repositories, under explicit developer control."
	}
	return ""
}

// detectHeadingBullets collects bullet lines that appear under a heading whose
// title contains any of the given keywords, across all docs.
func detectHeadingBullets(files []RawFile, keywords ...string) []string {
	var out []string
	for _, f := range files {
		if !isMarkdownPath(f.Path) || f.Content == "" {
			continue
		}
		active := false
		for _, ln := range strings.Split(f.Content, "\n") {
			if strings.HasPrefix(ln, "#") {
				h := strings.ToLower(ln)
				active = false
				for _, k := range keywords {
					if strings.Contains(h, k) {
						active = true
						break
					}
				}
				continue
			}
			if active {
				t := strings.TrimSpace(ln)
				if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
					out = append(out, strings.TrimSpace(stripMarkdown(t[2:])))
				}
			}
		}
	}
	return topN(dedupeStrings(out), 8)
}
