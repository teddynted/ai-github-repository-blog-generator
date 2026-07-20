package contentanalytics

import (
	"strconv"
	"strings"
	"time"
)

const isoDate = "2006-01-02"

// dateOf formats a time as an ISO date.
func dateOf(t time.Time) Date { return t.UTC().Format(isoDate) }

// parseDate parses an ISO date, defaulting to the zero time on error.
func parseDate(d Date) time.Time {
	t, err := time.Parse(isoDate, d)
	if err != nil {
		return time.Time{}
	}
	return t
}

// addDays returns the ISO date n days from d (n may be negative).
func addDays(d Date, n int) Date {
	t := parseDate(d)
	if t.IsZero() {
		return d
	}
	return dateOf(t.AddDate(0, 0, n))
}

// daysBetween returns b-a in whole days.
func daysBetween(a, b Date) int {
	ta, tb := parseDate(a), parseDate(b)
	if ta.IsZero() || tb.IsZero() {
		return 0
	}
	return int(tb.Sub(ta).Hours() / 24)
}

// weekKey returns the ISO date of the Monday of d's week (weekly-snapshot key).
func weekKey(d Date) Date {
	t := parseDate(d)
	if t.IsZero() {
		return d
	}
	offset := (int(t.Weekday()) + 6) % 7 // Monday=0
	return dateOf(t.AddDate(0, 0, -offset))
}

// monthKey returns the ISO date of the first day of d's month.
func monthKey(d Date) Date {
	t := parseDate(d)
	if t.IsZero() {
		return d
	}
	return dateOf(time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC))
}

// periodKey maps a date to its snapshot key for the given period.
func periodKey(period Period, d Date) Date {
	switch period {
	case PeriodWeekly:
		return weekKey(d)
	case PeriodMonthly:
		return monthKey(d)
	default:
		return d
	}
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

// round2 rounds to two decimal places.
func round2(f float64) float64 {
	return float64(int64(f*100+copysign(0.5, f))) / 100
}

func copysign(mag, sign float64) float64 {
	if sign < 0 {
		return -mag
	}
	return mag
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

// direction classifies a percentage change.
func direction(changePct float64) string {
	switch {
	case changePct > 1:
		return "up"
	case changePct < -1:
		return "down"
	default:
		return "flat"
	}
}

// safeDiv divides, returning 0 when the denominator is 0.
func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// itoa/atoiSafe are small conversion helpers used by adapters.
func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func atoiSafe(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
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
