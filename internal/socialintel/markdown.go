package socialintel

import (
	"fmt"
	"strings"
)

// Markdown renders a morning briefing as a production-ready document.
func (b MorningBriefing) Markdown() string {
	var s strings.Builder
	fmt.Fprintf(&s, "# Morning Briefing — %s\n\n", b.Date)
	fmt.Fprintf(&s, "**%s**\n\n", b.Headline)

	if len(b.Alerts) > 0 {
		fmt.Fprintf(&s, "> %s\n\n", strings.Join(b.Alerts, "  \n> "))
	}

	s.WriteString("## Platform Performance\n\n")
	s.WriteString("| Platform | Followers | Δ Followers | Views | Δ Views | Engagement |\n|---|---|---|---|---|---|\n")
	for _, p := range b.Platforms {
		fmt.Fprintf(&s, "| %s | %s | %+d | %s | %+d | %.2f%% |\n",
			p.Platform, comma(p.Followers), p.FollowerDelta, comma(p.Views), p.ViewsDelta, p.EngagementRate)
	}
	fmt.Fprintf(&s, "\n**New followers yesterday:** %d\n\n", b.NewFollowers)

	writeCross(&s, b.CrossPlatform)

	if len(b.BestContent) > 0 {
		s.WriteString("## Top Content\n\n")
		for _, c := range b.BestContent {
			fmt.Fprintf(&s, "- **%s** (%s) — score %.0f/100, engagement %.1f%%, virality %.0f, evergreen %.0f\n",
				firstNonEmpty(c.Title, c.ContentID), c.Platform, c.Score, c.EngagementRate, c.ViralityScore, c.EvergreenScore)
		}
		s.WriteString("\n")
	}

	if len(b.TopConversion) > 0 {
		s.WriteString("## Best Subscriber Conversion\n\n")
		for _, c := range b.TopConversion {
			fmt.Fprintf(&s, "- **%s** — +%d subs from %s views (%.2f%% conversion)\n",
				firstNonEmpty(c.Title, c.ContentID), c.NetSubscribers, comma(c.Views), c.ConversionRate)
		}
		s.WriteString("\n")
	}

	if len(b.TrendingTopics) > 0 {
		fmt.Fprintf(&s, "**Trending topics:** %s\n\n", strings.Join(b.TrendingTopics, ", "))
	}

	writeList(&s, "Insights", insightLines(b.Insights))
	writeList(&s, "Recommendations", recLines(b.Recommendations))
	writeList(&s, "Publishing opportunities", b.PublishingWindows)
	return s.String()
}

func writeCross(s *strings.Builder, cp CrossPlatform) {
	s.WriteString("## Cross-Platform\n\n")
	fmt.Fprintf(s, "- **Best:** %s · **Fastest growing:** %s · **Highest engagement:** %s\n",
		orDash(cp.BestPlatform), orDash(cp.FastestGrowing), orDash(cp.HighestEngagement))
	if cp.HighestConversion != "" || cp.HighestRetention != "" {
		fmt.Fprintf(s, "- **Highest conversion:** %s · **Highest retention:** %s\n", orDash(cp.HighestConversion), orDash(cp.HighestRetention))
	}
	s.WriteString("\n")
}

func writeList(s *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(s, "## %s\n\n", title)
	for _, it := range items {
		fmt.Fprintf(s, "- %s\n", it)
	}
	s.WriteString("\n")
}

func insightLines(ins []BusinessInsight) []string {
	var out []string
	for _, i := range ins {
		out = append(out, "["+i.Kind+"] "+i.Summary)
	}
	return out
}

func recLines(recs []Recommendation) []string {
	var out []string
	for _, r := range recs {
		out = append(out, "["+r.Category+"] "+r.Action+" — "+r.Rationale)
	}
	return out
}

func orDash(p Platform) string {
	if p == "" {
		return "—"
	}
	return string(p)
}

// comma formats an integer with thousands separators.
func comma(n int64) string {
	s := itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, ",")
	if neg {
		return "-" + out
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
