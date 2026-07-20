package contentoptimizer

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// round2 rounds to two decimal places.
func round2(f float64) float64 {
	if f < 0 {
		return -round2(-f)
	}
	return float64(int64(f*100+0.5)) / 100
}

// clampFloat bounds v to [lo, hi].
func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// pct returns the percentage change from prev to cur.
func pct(cur, prev float64) float64 {
	if prev == 0 {
		if cur == 0 {
			return 0
		}
		return 100
	}
	return round2((cur - prev) / prev * 100)
}

// mean averages a slice (0 for empty).
func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// percentile returns the p-quantile (0..1) of xs using nearest-rank on a copy.
func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 1 {
		return cp[len(cp)-1]
	}
	idx := int(p * float64(len(cp)-1))
	return cp[idx]
}

// expectedImpact maps a confidence to a coarse impact label.
func expectedImpact(confidence float64) string {
	switch {
	case confidence >= 0.75:
		return "high"
	case confidence >= 0.5:
		return "medium"
	default:
		return "low"
	}
}

// supportConfidence blends sample size and effect separation into a 0–1 score.
// It is deterministic and explainable: more supporting records and a larger gap
// from the baseline both raise confidence.
func supportConfidence(support int, separation float64) float64 {
	// Sample-size factor saturates around 8 supporting records.
	sizeF := clampFloat(float64(support)/8.0, 0, 1)
	// Separation factor: how far above/below baseline (as a fraction), capped.
	sepF := clampFloat(separation, 0, 1)
	return round2(clampFloat(0.35+0.4*sizeF+0.25*sepF, 0, 0.99))
}

// dedupeStr removes duplicates preserving order.
func dedupeStr(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// topN returns the first n elements (or fewer).
func topN[T any](in []T, n int) []T {
	if n < 0 {
		n = 0
	}
	if len(in) > n {
		return in[:n]
	}
	return in
}

// shortID makes a stable short id from parts (for patterns/recommendations/runs).
func shortID(parts ...string) string {
	h := sha1.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])[:12]
}

// join renders a comma-separated list.
func join(ss []string) string { return strings.Join(ss, ", ") }

// sortStable is a small generic stable-sort wrapper for readability.
func sortStable[T any](in []T, less func(a, b T) bool) {
	sort.SliceStable(in, func(i, j int) bool { return less(in[i], in[j]) })
}

// sb is a tiny line-oriented string builder for Markdown rendering.
type sb struct{ b strings.Builder }

func (s *sb) line(str string) { s.b.WriteString(str); s.b.WriteByte('\n') }
func (s *sb) linef(format string, args ...any) {
	fmt.Fprintf(&s.b, format, args...)
	s.b.WriteByte('\n')
}
func (s *sb) String() string { return s.b.String() }
