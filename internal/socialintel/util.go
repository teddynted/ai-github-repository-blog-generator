package socialintel

import (
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// dateOf formats a time as an ISO date.
func dateOf(t time.Time) Date { return t.UTC().Format(dateLayout) }

// parseDate parses an ISO date (zero time on error).
func parseDate(d Date) time.Time {
	t, err := time.Parse(dateLayout, d)
	if err != nil {
		return time.Time{}
	}
	return t
}

// addDays returns the ISO date n days from d.
func addDays(d Date, n int) Date {
	t := parseDate(d)
	if t.IsZero() {
		return d
	}
	return dateOf(t.AddDate(0, 0, n))
}

// daysBetween returns the number of days from a to b (b - a).
func daysBetween(a, b Date) int {
	ta, tb := parseDate(a), parseDate(b)
	if ta.IsZero() || tb.IsZero() {
		return 0
	}
	return int(tb.Sub(ta).Hours() / 24)
}

// pct returns 100*(cur-prev)/prev, guarding against divide-by-zero.
func pct(cur, prev float64) float64 {
	if prev == 0 {
		if cur == 0 {
			return 0
		}
		return 100
	}
	return round2((cur - prev) / prev * 100)
}

func round2(f float64) float64 {
	return float64(int64(f*100+sign(f)*0.5)) / 100
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func dedupeStr(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	return out
}

func topN[T any](list []T, n int) []T {
	if n <= 0 || len(list) <= n {
		return list
	}
	return list[:n]
}

// atoiSafe parses an integer string (many social APIs return counts as strings),
// returning 0 on error.
func atoiSafe(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var n int64
	neg := false
	for i, r := range s {
		if i == 0 && r == '-' {
			neg = true
			continue
		}
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int64(r-'0')
	}
	if neg {
		return -n
	}
	return n
}

func join(items []string) string { return strings.Join(items, ",") }

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [24]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
