package socialintel

// Analytics computes trends and growth from the immutable snapshot history. It
// is deterministic — the same snapshots always produce the same result.
type Analytics struct {
	Repo   Repository
	Config Config
}

// metricAccessor extracts a metric value from an account snapshot.
type metricAccessor func(AccountSnapshot) float64

var accountMetrics = map[string]metricAccessor{
	"followers":   func(s AccountSnapshot) float64 { return float64(s.Followers) },
	"views":       func(s AccountSnapshot) float64 { return float64(s.Views) },
	"impressions": func(s AccountSnapshot) float64 { return float64(s.Impressions) },
	"reach":       func(s AccountSnapshot) float64 { return float64(s.Reach) },
}

// Series builds a historical series for a platform metric (retention-bounded).
func (a Analytics) Series(p Platform, metric string) HistoricalSeries {
	acc := accountMetrics[metric]
	if acc == nil {
		return HistoricalSeries{Metric: metric}
	}
	snaps := a.bounded(a.Repo.AccountSeries(p))
	pts := make([]Point, 0, len(snaps))
	for _, s := range snaps {
		pts = append(pts, Point{Date: s.Date, Value: acc(s)})
	}
	return HistoricalSeries{Metric: metric, Points: pts}
}

// Growth computes DoD/WoW/MoM growth, a 7-day rolling average, velocity, and
// momentum for a platform metric.
func (a Analytics) Growth(p Platform, metric string) GrowthMetrics {
	s := a.Series(p, metric)
	g := GrowthMetrics{Platform: p, Metric: metric}
	n := len(s.Points)
	if n == 0 {
		return g
	}
	cur := s.Points[n-1]
	g.Current = cur.Value
	g.DayOverDay = a.delta(s, n-1, 1)
	g.WeekOverWeek = a.delta(s, n-1, 7)
	g.MonthOverMonth = a.delta(s, n-1, 30)
	g.Rolling7Avg = rollingAvg(s.Points, 7)
	g.Velocity = velocity(s.Points)
	g.Momentum = momentum(s.Points)
	return g
}

// Trend summarizes the direction of a metric over the retention window.
func (a Analytics) Trend(p Platform, metric string) Trend {
	s := a.Series(p, metric)
	t := Trend{Metric: metric, Direction: "flat", Series: s.Points}
	if len(s.Points) < 2 {
		return t
	}
	first := s.Points[0].Value
	last := s.Points[len(s.Points)-1].Value
	t.ChangePct = pct(last, first)
	switch {
	case t.ChangePct > 1:
		t.Direction = "up"
	case t.ChangePct < -1:
		t.Direction = "down"
	}
	return t
}

// delta compares the value at index i to the value `back` days earlier (by date,
// not by index, so gaps in the series don't distort the comparison).
func (a Analytics) delta(s HistoricalSeries, i, back int) Delta {
	cur := s.Points[i]
	target := addDays(cur.Date, -back)
	prev := valueOnOrBefore(s.Points, target)
	return Delta{Absolute: round2(cur.Value - prev), Percent: pct(cur.Value, prev)}
}

// bounded trims the series to the retention window.
func (a Analytics) bounded(snaps []AccountSnapshot) []AccountSnapshot {
	if a.Config.RetentionDays <= 0 || len(snaps) == 0 {
		return snaps
	}
	cutoff := addDays(snaps[len(snaps)-1].Date, -a.Config.RetentionDays)
	var out []AccountSnapshot
	for _, s := range snaps {
		if s.Date >= cutoff {
			out = append(out, s)
		}
	}
	return out
}

// valueOnOrBefore returns the value of the latest point at/on-or-before target.
func valueOnOrBefore(pts []Point, target Date) float64 {
	val := 0.0
	found := false
	for _, p := range pts {
		if p.Date <= target {
			val = p.Value
			found = true
		}
	}
	if !found && len(pts) > 0 {
		return pts[0].Value // fall back to the earliest known value
	}
	return val
}

// rollingAvg averages the last n points.
func rollingAvg(pts []Point, n int) float64 {
	if len(pts) == 0 {
		return 0
	}
	if n > len(pts) {
		n = len(pts)
	}
	var vals []float64
	for _, p := range pts[len(pts)-n:] {
		vals = append(vals, p.Value)
	}
	return round2(mean(vals))
}

// velocity is the average day-over-day change across the series.
func velocity(pts []Point) float64 {
	if len(pts) < 2 {
		return 0
	}
	total := pts[len(pts)-1].Value - pts[0].Value
	days := float64(daysBetween(pts[0].Date, pts[len(pts)-1].Date))
	if days <= 0 {
		days = float64(len(pts) - 1)
	}
	return round2(total / days)
}

// momentum compares recent velocity to earlier velocity.
func momentum(pts []Point) string {
	if len(pts) < 4 {
		return "flat"
	}
	mid := len(pts) / 2
	early := velocity(pts[:mid+1])
	recent := velocity(pts[mid:])
	switch {
	case recent > early*1.1 && recent > 0:
		return "accelerating"
	case recent < early*0.9:
		return "decelerating"
	case early == 0 && recent == 0:
		return "flat"
	default:
		return "steady"
	}
}
