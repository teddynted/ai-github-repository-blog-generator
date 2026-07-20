package contentoptimizer

import "fmt"

// DefaultPatternDetector detects winning/losing content patterns deterministically
// from the analytics. It ranks records by their blended score, isolates the top
// and bottom quantiles, and reports the metric signals that distinguish them —
// always grounded in the records, with a confidence from sample size + effect.
type DefaultPatternDetector struct {
	Config Config
}

// NewPatternDetector wires a pattern detector.
func NewPatternDetector(cfg Config) *DefaultPatternDetector {
	return &DefaultPatternDetector{Config: cfg}
}

// baseline holds mean metric values across all records.
type baseline struct {
	ctr, retention, engagement, watch, score float64
	n                                        int
}

func computeBaseline(records []PerformanceRecord) baseline {
	var b baseline
	if len(records) == 0 {
		return b
	}
	var ctr, ret, eng, watch, score []float64
	for _, r := range records {
		ctr = append(ctr, r.CTR)
		ret = append(ret, r.RetentionPct)
		eng = append(eng, r.EngagementRate)
		watch = append(watch, float64(r.WatchMinutes))
		score = append(score, r.Score)
	}
	b = baseline{ctr: mean(ctr), retention: mean(ret), engagement: mean(eng), watch: mean(watch), score: mean(score), n: len(records)}
	return b
}

func meanMetric(records []PerformanceRecord, name string) float64 {
	var xs []float64
	for _, r := range records {
		if v, ok := r.metric(name); ok {
			xs = append(xs, v)
		}
	}
	return mean(xs)
}

// Detect returns grounded winning and losing patterns.
func (d *DefaultPatternDetector) Detect(input AnalyticsInput) (winning, losing []Pattern) {
	records := input.Records
	if len(records) == 0 {
		return nil, nil
	}
	base := computeBaseline(records)

	scores := make([]float64, 0, len(records))
	for _, r := range records {
		scores = append(scores, r.Score)
	}
	winCut := percentile(scores, d.Config.winningPct())
	loseCut := percentile(scores, d.Config.losingPct())

	var winners, losers []PerformanceRecord
	for _, r := range records {
		switch {
		case r.Score >= winCut:
			winners = append(winners, r)
		case r.Score <= loseCut:
			losers = append(losers, r)
		}
	}

	if p, ok := d.winningChain(winners, base); ok {
		winning = append(winning, p)
	}
	winning = append(winning, d.topicPatterns(input, winners, PatternWinning)...)

	if p, ok := d.losingChain(losers, base); ok {
		losing = append(losing, p)
	}
	losing = append(losing, d.topicPatterns(input, losers, PatternLosing)...)

	return winning, losing
}

// winningChain builds the High-CTR → Strong-Hook → Long-Watch → Subscriber-Growth
// pattern when the top cohort measurably exceeds the baseline.
func (d *DefaultPatternDetector) winningChain(winners []PerformanceRecord, base baseline) (Pattern, bool) {
	if len(winners) < d.Config.minSupport() {
		return Pattern{}, false
	}
	wCTR := meanMetric(winners, "ctr")
	wRet := meanMetric(winners, "retentionPct")
	wEng := meanMetric(winners, "engagementRate")
	wWatch := meanMetric(winners, "watchMinutes")
	wScore := meanMetric(winners, "score")

	var chain, evidence []string
	if wCTR > base.ctr && base.ctr > 0 {
		chain = append(chain, "High CTR")
		evidence = append(evidence, fmt.Sprintf("CTR %.2f%% vs baseline %.2f%%", wCTR, base.ctr))
	}
	if wRet >= base.retention && base.retention > 0 {
		chain = append(chain, "Strong retention (hook)")
		evidence = append(evidence, fmt.Sprintf("retention %.1f%% vs baseline %.1f%%", wRet, base.retention))
	}
	if wWatch >= base.watch && base.watch > 0 {
		chain = append(chain, "Longer watch time")
		evidence = append(evidence, fmt.Sprintf("watch %.0fm vs baseline %.0fm", wWatch, base.watch))
	}
	if wEng > base.engagement {
		chain = append(chain, "High engagement")
		evidence = append(evidence, fmt.Sprintf("engagement %.2f%% vs baseline %.2f%%", wEng, base.engagement))
	}
	if len(chain) < 2 {
		return Pattern{}, false // not enough distinguishing signal to be honest
	}
	chain = append(chain, "→ replicate this pattern")

	sep := clampFloat(pct(wScore, base.score)/100, 0, 1)
	p := Pattern{
		ID:          shortID("win", "chain"),
		Kind:        PatternWinning,
		Name:        "High-performing content signature",
		SignalChain: chain,
		Description: "Top-quartile content shares elevated click-through, retention and engagement.",
		Support:     len(winners),
		Confidence:  supportConfidence(len(winners), sep),
		Evidence:    evidence,
		Examples:    exampleTitles(winners, 3),
	}
	return p, true
}

// losingChain builds the Low-Engagement → Weak-Hook → Poor-Retention pattern.
func (d *DefaultPatternDetector) losingChain(losers []PerformanceRecord, base baseline) (Pattern, bool) {
	if len(losers) < d.Config.minSupport() {
		return Pattern{}, false
	}
	lEng := meanMetric(losers, "engagementRate")
	lRet := meanMetric(losers, "retentionPct")
	lCTR := meanMetric(losers, "ctr")
	lScore := meanMetric(losers, "score")

	var chain, evidence []string
	if lEng < base.engagement {
		chain = append(chain, "Low engagement")
		evidence = append(evidence, fmt.Sprintf("engagement %.2f%% vs baseline %.2f%%", lEng, base.engagement))
	}
	if lCTR < base.ctr && base.ctr > 0 {
		chain = append(chain, "Weak hook (low CTR)")
		evidence = append(evidence, fmt.Sprintf("CTR %.2f%% vs baseline %.2f%%", lCTR, base.ctr))
	}
	if lRet < base.retention && base.retention > 0 {
		chain = append(chain, "Poor retention")
		evidence = append(evidence, fmt.Sprintf("retention %.1f%% vs baseline %.1f%%", lRet, base.retention))
	}
	if len(chain) < 2 {
		return Pattern{}, false
	}
	chain = append(chain, "→ try an alternative approach")

	sep := clampFloat(pct(base.score, lScore)/100, 0, 1)
	p := Pattern{
		ID:          shortID("lose", "chain"),
		Kind:        PatternLosing,
		Name:        "Underperforming content signature",
		SignalChain: chain,
		Description: "Bottom-quartile content shows weak hooks and low retention/engagement.",
		Support:     len(losers),
		Confidence:  supportConfidence(len(losers), sep),
		Evidence:    evidence,
		Examples:    exampleTitles(losers, 3),
	}
	return p, true
}

// topicPatterns emits per-tag patterns grounded either in record tags (preferred)
// or the M17 aggregate TopTags. Only tags meeting minimum support are reported.
func (d *DefaultPatternDetector) topicPatterns(input AnalyticsInput, cohort []PerformanceRecord, kind PatternKind) []Pattern {
	// Prefer record-level tags when present.
	counts := map[string]int{}
	for _, r := range cohort {
		for _, t := range r.Tags {
			counts[t]++
		}
	}
	var out []Pattern
	if len(counts) > 0 {
		for tag, c := range counts {
			if c < d.Config.minSupport() {
				continue
			}
			out = append(out, tagPattern(tag, c, kind, fmt.Sprintf("%d of the %s cohort use this tag", c, kind)))
		}
		return rankPatterns(out, d.Config.topN())
	}
	// Fall back to aggregate signals (winning only — aggregates rank by reach).
	if kind == PatternWinning {
		for _, t := range topN(input.TopTags, d.Config.topN()) {
			if t.Publications < d.Config.minSupport() {
				continue
			}
			p := tagPattern(t.Tag, t.Publications, kind,
				fmt.Sprintf("%d publications, %.2f%% engagement, %d views", t.Publications, t.EngagementRate, t.Views))
			out = append(out, p)
		}
	}
	return rankPatterns(out, d.Config.topN())
}

func tagPattern(tag string, support int, kind PatternKind, evidence string) Pattern {
	verb := "Favor"
	if kind == PatternLosing {
		verb = "Reconsider"
	}
	return Pattern{
		ID:          shortID(string(kind), "tag", tag),
		Kind:        kind,
		Name:        fmt.Sprintf("%s topic: %s", verb, tag),
		SignalChain: []string{"Topic: " + tag, fmt.Sprintf("→ %s in future content", verb)},
		Description: fmt.Sprintf("The %q topic recurs in the %s cohort.", tag, kind),
		Support:     support,
		Confidence:  supportConfidence(support, 0.5),
		Evidence:    []string{evidence},
	}
}

func exampleTitles(records []PerformanceRecord, n int) []string {
	var out []string
	for _, r := range records {
		title := r.Title
		if title == "" {
			title = r.PublicationID
		}
		out = append(out, title)
	}
	return topN(dedupeStr(out), n)
}

func rankPatterns(ps []Pattern, n int) []Pattern {
	sortStable(ps, func(a, b Pattern) bool {
		if a.Support != b.Support {
			return a.Support > b.Support
		}
		return a.Confidence > b.Confidence
	})
	return topN(ps, n)
}
