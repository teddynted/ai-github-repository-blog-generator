package contentanalytics

import "time"

// Snapshotter derives immutable weekly and monthly historical snapshots from the
// daily snapshots. A roll-up takes the most recent daily snapshot within the
// bucket (weekly = ISO week starting Monday, monthly = calendar month) as the
// period's representative cumulative measurement and stores it under the bucket
// key. Roll-ups are idempotent — an existing bucket is never overwritten — so
// the honest historical record is preserved.
type Snapshotter struct {
	Repo Repository
	Now  Clock
}

// NewSnapshotter wires a snapshotter.
func NewSnapshotter(repo Repository, now Clock) *Snapshotter {
	if now == nil {
		now = time.Now
	}
	return &Snapshotter{Repo: repo, Now: now}
}

// RollUp writes weekly and monthly roll-up snapshots for the bucket that
// contains date. Returns the number of roll-up snapshots written.
func (s *Snapshotter) RollUp(date Date) int {
	return s.rollUpPeriod(PeriodWeekly, date) + s.rollUpPeriod(PeriodMonthly, date)
}

// RollUpRange rolls up every bucket touched by [from, to] for both cadences.
func (s *Snapshotter) RollUpRange(from, to Date) int {
	written := 0
	seenWeek := map[Date]bool{}
	seenMonth := map[Date]bool{}
	for d := from; d <= to; d = addDays(d, 1) {
		if wk := weekKey(d); !seenWeek[wk] {
			seenWeek[wk] = true
			written += s.rollUpPeriod(PeriodWeekly, d)
		}
		if mk := monthKey(d); !seenMonth[mk] {
			seenMonth[mk] = true
			written += s.rollUpPeriod(PeriodMonthly, d)
		}
		if daysBetween(from, d) > 3660 { // safety bound
			break
		}
	}
	return written
}

func (s *Snapshotter) rollUpPeriod(period Period, date Date) int {
	bucketStart := periodKey(period, date)
	bucketEnd := bucketEndOf(period, bucketStart)
	written := 0

	for _, pub := range s.Repo.Publications() {
		if s.Repo.SnapshotExists(pub.ID, period, bucketStart) {
			continue
		}
		rep, ok := latestDailyInBucket(s.Repo.Snapshots(pub.ID, PeriodDaily), bucketStart, bucketEnd)
		if !ok {
			continue
		}
		rep.Period = period
		rep.Date = bucketStart
		if rep.CollectedAt.IsZero() {
			rep.CollectedAt = s.Now()
		}
		if err := s.Repo.SaveSnapshot(rep); err == nil {
			written++
		}
	}
	return written
}

// latestDailyInBucket returns the newest daily snapshot with a date in
// [start, end], and whether one was found.
func latestDailyInBucket(daily []MetricsSnapshot, start, end Date) (MetricsSnapshot, bool) {
	var best MetricsSnapshot
	found := false
	for _, s := range daily {
		if s.Date >= start && s.Date <= end {
			if !found || s.Date > best.Date {
				best, found = s, true
			}
		}
	}
	return best, found
}

// bucketEndOf returns the last date in a period bucket that starts at start.
func bucketEndOf(period Period, start Date) Date {
	switch period {
	case PeriodWeekly:
		return addDays(start, 6)
	case PeriodMonthly:
		t := parseDate(start)
		if t.IsZero() {
			return start
		}
		return dateOf(t.AddDate(0, 1, -1))
	default:
		return start
	}
}
