package socialintel

import (
	"log/slog"
	"time"
)

// LogMonitor emits observability signals via slog, standing in for CloudWatch.
// A CloudWatch EMF adapter implements the same Monitor interface. It never logs
// credentials.
type LogMonitor struct {
	Logger *slog.Logger
}

func (m LogMonitor) log() *slog.Logger {
	if m.Logger != nil {
		return m.Logger
	}
	return slog.Default()
}

func (m LogMonitor) Count(name string, value float64, dims map[string]string) {
	m.log().Info("metric.count", slog.String("name", name), slog.Float64("value", value), slog.Any("dims", dims))
}

func (m LogMonitor) Duration(name string, d time.Duration, dims map[string]string) {
	m.log().Info("metric.duration", slog.String("name", name), slog.Int64("ms", d.Milliseconds()), slog.Any("dims", dims))
}

func (m LogMonitor) Error(name string, err error, dims map[string]string) {
	m.log().Error("metric.error", slog.String("name", name), slog.String("code", errCode(err)), slog.Any("dims", dims))
}

// nopMonitor discards signals (default when none is configured).
type nopMonitor struct{}

func (nopMonitor) Count(string, float64, map[string]string)          {}
func (nopMonitor) Duration(string, time.Duration, map[string]string) {}
func (nopMonitor) Error(string, error, map[string]string)            {}
