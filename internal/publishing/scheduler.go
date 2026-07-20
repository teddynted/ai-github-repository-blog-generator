package publishing

import "time"

// Scheduler decides when a publication runs, honouring schedule mode, time
// zones, business-hours windows, and recurrence.
type Scheduler struct {
	Now Clock
}

// NextRun computes the next time a publication should run for a schedule.
func (s Scheduler) NextRun(sc Schedule, from time.Time) time.Time {
	switch sc.Mode {
	case ScheduleImmediate, "":
		return from
	case ScheduleDelay:
		return s.applyWindow(sc, from.Add(sc.Delay))
	case ScheduleAt:
		at := sc.At
		if at.Before(from) {
			at = from
		}
		return s.applyWindow(sc, at)
	case ScheduleRecurring:
		// Minimal recurrence: next day at the window start (a real cron parser is
		// a future enhancement).
		return s.applyWindow(sc, from.Add(24*time.Hour))
	default:
		return from
	}
}

// Due reports whether a scheduled publication's time has arrived.
func (s Scheduler) Due(scheduledFor time.Time) bool {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	return !scheduledFor.After(now())
}

// applyWindow shifts t forward into the business-hours window (in the schedule's
// time zone) when one is configured.
func (s Scheduler) applyWindow(sc Schedule, t time.Time) time.Time {
	if sc.Window == nil {
		return t
	}
	loc := time.UTC
	if sc.TimeZone != "" {
		if l, err := time.LoadLocation(sc.TimeZone); err == nil {
			loc = l
		}
	}
	lt := t.In(loc)
	start, end := sc.Window.StartHour, sc.Window.EndHour
	if start < 0 || start > 23 || end <= start || end > 24 {
		return t // invalid window → no constraint
	}
	h := lt.Hour()
	switch {
	case h < start:
		// Move to start hour today.
		return time.Date(lt.Year(), lt.Month(), lt.Day(), start, 0, 0, 0, loc)
	case h >= end:
		// Move to start hour tomorrow.
		n := lt.Add(24 * time.Hour)
		return time.Date(n.Year(), n.Month(), n.Day(), start, 0, 0, 0, loc)
	default:
		return t // already within the window
	}
}
