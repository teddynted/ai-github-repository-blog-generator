package publishing

import (
	"fmt"
	"strings"
)

// Report renders a set of publications as a production-ready Markdown summary:
// per-platform status, URL, retries, duration, and the audit trail.
func Report(pubs []*Publication) string {
	var b strings.Builder
	b.WriteString("# Publication Report\n\n")

	published, failed, scheduled := 0, 0, 0
	for _, p := range pubs {
		switch p.Status {
		case StatusPublished:
			published++
		case StatusFailed:
			failed++
		case StatusScheduled:
			scheduled++
		}
	}
	fmt.Fprintf(&b, "_%d target(s) · %d published · %d scheduled · %d failed_\n\n", len(pubs), published, scheduled, failed)

	b.WriteString("| Platform | Status | URL | Retries | Duration |\n|---|---|---|---|---|\n")
	for _, p := range pubs {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %dms |\n",
			p.Platform, p.Status, firstNonEmpty(p.URL, "—"), p.RetryCount, p.DurationMs)
	}
	b.WriteString("\n")

	for _, p := range pubs {
		writePublication(&b, p)
	}
	return b.String()
}

func writePublication(b *strings.Builder, p *Publication) {
	fmt.Fprintf(b, "---\n\n## %s — %s\n\n", p.Platform, p.Status)
	fmt.Fprintf(b, "- **Content:** %s (%s)\n", p.ContentID, p.ContentType)
	if p.URL != "" {
		fmt.Fprintf(b, "- **URL:** %s\n", p.URL)
	}
	if p.PlatformID != "" {
		fmt.Fprintf(b, "- **Platform ID:** %s\n", p.PlatformID)
	}
	if p.ApprovalRef != "" {
		fmt.Fprintf(b, "- **Approval:** %s\n", p.ApprovalRef)
	}
	if p.ReleaseVersion != "" {
		fmt.Fprintf(b, "- **Release:** %s\n", p.ReleaseVersion)
	}
	if p.ScheduledFor != nil {
		fmt.Fprintf(b, "- **Scheduled for:** %s\n", p.ScheduledFor.Format("2006-01-02 15:04 MST"))
	}
	fmt.Fprintf(b, "- **Retries:** %d · **Duration:** %dms\n", p.RetryCount, p.DurationMs)
	if len(p.Errors) > 0 {
		fmt.Fprintf(b, "- **Errors:** %s\n", strings.Join(p.Errors, "; "))
	}

	if len(p.Attempts) > 0 {
		b.WriteString("\n**Attempts:**\n")
		for _, a := range p.Attempts {
			outcome := "✓"
			if !a.Success {
				outcome = "✗"
			}
			fmt.Fprintf(b, "- %s #%d (%dms)%s\n", outcome, a.Number, a.Duration.Milliseconds(), errSuffix(a.Error))
		}
	}

	b.WriteString("\n**Audit trail:**\n")
	for _, e := range p.Audit {
		fmt.Fprintf(b, "- `%s` **%s**", e.Timestamp.Format("15:04:05"), e.Action)
		if e.From != "" && e.From != e.To {
			fmt.Fprintf(b, " (%s → %s)", e.From, e.To)
		}
		if e.Detail != "" {
			fmt.Fprintf(b, " — %s", e.Detail)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func errSuffix(e string) string {
	if e == "" {
		return ""
	}
	return " — " + e
}
