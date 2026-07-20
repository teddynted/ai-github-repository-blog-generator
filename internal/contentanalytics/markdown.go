package contentanalytics

import (
	"fmt"
	"strings"
)

// Markdown renders a Report as a production-ready document.
func (r Report) Markdown() string {
	var s strings.Builder
	fmt.Fprintf(&s, "# Content Analytics Report — %s\n\n", r.Window)
	fmt.Fprintf(&s, "%s\n\n", r.ExecutiveSummary)

	s.WriteString("## Platform Summary\n\n")
	s.WriteString("| Platform | Publications | Views | Engagements | Engagement | Watch (min) |\n|---|---|---|---|---|---|\n")
	for _, p := range r.Platforms {
		fmt.Fprintf(&s, "| %s | %d | %s | %s | %.2f%% | %s |\n",
			p.Platform, p.Publications, comma(p.Views), comma(p.Engagements), p.EngagementRate, comma(p.WatchTimeMinutes))
	}
	s.WriteString("\n")

	writeContentTable(&s, "Top Articles", r.TopArticles)
	writeContentTable(&s, "Top Videos", r.TopVideos)

	s.WriteString("## Engagement Breakdown\n\n")
	b := r.EngagementBreakdown
	fmt.Fprintf(&s, "- Likes/Reactions: **%s**\n- Comments/Replies: **%s**\n- Shares/Reposts: **%s**\n- Saves/Bookmarks: **%s**\n- **Total: %s**\n\n",
		comma(b.Likes), comma(b.Comments), comma(b.Shares), comma(b.Saves), comma(b.Total))

	if len(r.GrowthTrends) > 0 {
		s.WriteString("## Growth Trends (views)\n\n")
		s.WriteString("| Platform | Current | DoD | WoW | MoM | Direction |\n|---|---|---|---|---|---|\n")
		for _, g := range r.GrowthTrends {
			fmt.Fprintf(&s, "| %s | %s | %+.1f%% | %+.1f%% | %+.1f%% | %s |\n",
				g.Platform, comma(int64(g.Current)), g.DayOverDay.Percent, g.WeekOverWeek.Percent, g.MonthOverMonth.Percent, g.Direction)
		}
		s.WriteString("\n")
	}

	if len(r.WorstPerforming) > 0 {
		s.WriteString("## Needs Attention\n\n")
		for _, c := range r.WorstPerforming {
			fmt.Fprintf(&s, "- **%s** (%s) — score %.0f/100, %.2f%% engagement\n", c.Title, c.Platform, c.Score, c.EngagementRate)
		}
		s.WriteString("\n")
	}

	if len(r.Recommendations) > 0 {
		s.WriteString("## Recommendations\n\n")
		for _, rec := range r.Recommendations {
			fmt.Fprintf(&s, "- %s\n", rec)
		}
		s.WriteString("\n")
	}

	q := r.DataQuality
	s.WriteString("## Data Quality\n\n")
	fmt.Fprintf(&s, "- Publications: **%d** · Snapshots: **%d** · Partial: **%d** · Completeness: **%.1f%%**\n",
		q.Publications, q.Snapshots, q.PartialSnapshots, q.CompletenessScore)
	if len(q.MissingPlatforms) > 0 {
		fmt.Fprintf(&s, "- Missing platforms: %s\n", strings.Join(q.MissingPlatforms, ", "))
	}
	return s.String()
}

func writeContentTable(s *strings.Builder, title string, items []ContentPerformance) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(s, "## %s\n\n", title)
	s.WriteString("| # | Title | Platform | Views | Engagement | Score |\n|---|---|---|---|---|---|\n")
	for _, c := range items {
		fmt.Fprintf(s, "| %d | %s | %s | %s | %.2f%% | %.0f |\n",
			c.Rank, c.Title, c.Platform, comma(c.Views), c.EngagementRate, c.Score)
	}
	s.WriteString("\n")
}
