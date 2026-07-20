package contentanalytics

import (
	"context"
	"log/slog"
	"time"
)

// CloudWatch metric names emitted by the collector + dashboard. These map to
// the custom metrics the milestone requires.
const (
	MetricTotalViews            = "TotalViews"
	MetricDailyViews            = "DailyViews"
	MetricEngagementRate        = "EngagementRate"
	MetricCTR                   = "CTR"
	MetricWatchTime             = "WatchTimeMinutes"
	MetricPublishingSuccessRate = "PublishingSuccessRate"
	MetricAPIFailures           = "APIFailures"
	MetricAPILatency            = "APILatencyMs"
	MetricCollectionDuration    = "CollectionDurationMs"
	MetricAuthFailures          = "AuthenticationFailures"
	MetricMissingAnalytics      = "MissingAnalytics"
	MetricFailedCollections     = "FailedCollections"
)

// LogMetricsPublisher writes metrics to the structured log, standing in for a
// real CloudWatch PutMetricData adapter (which would batch these into the
// CloudWatch API). It has the exact shape a real adapter needs, so swapping in
// the AWS SDK requires no caller changes.
type LogMetricsPublisher struct {
	Logger *slog.Logger
}

func (p LogMetricsPublisher) log() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.Default()
}

// Publish emits each metric as a structured log line under the namespace.
func (p LogMetricsPublisher) Publish(ctx context.Context, namespace string, metrics []Metric) error {
	l := p.log()
	for _, m := range metrics {
		attrs := []any{"namespace", namespace, "metric", m.Name, "value", m.Value, "unit", m.Unit}
		for k, v := range m.Dimensions {
			attrs = append(attrs, "dim."+k, v)
		}
		l.InfoContext(ctx, "cloudwatch.metric", attrs...)
	}
	return nil
}

// nopMetricsPublisher discards metrics (default when none is configured).
type nopMetricsPublisher struct{}

func (nopMetricsPublisher) Publish(context.Context, string, []Metric) error { return nil }

// metric is a small builder that stamps a timestamp + dimensions.
func metric(name string, value float64, unit string, dims map[string]string, ts time.Time) Metric {
	return Metric{Name: name, Value: value, Unit: unit, Dimensions: dims, Timestamp: ts}
}
