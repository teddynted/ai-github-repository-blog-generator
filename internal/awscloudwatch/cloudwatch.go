// Package awscloudwatch adapts the CloudWatch GetMetricData API to the idle
// detector's Metrics port: it returns per-period CPU and network datapoints for
// a single EC2 instance over a trailing window.
package awscloudwatch

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// API is the subset of the CloudWatch client this adapter uses.
type API interface {
	GetMetricData(ctx context.Context, in *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

// Client returns EC2 metrics for one instance. Period is the aggregation
// granularity in seconds (300 = 5 minutes, matching basic monitoring).
type Client struct {
	api        API
	instanceID string
	period     int32
}

// New builds a Client. A period <= 0 defaults to 300 seconds.
func New(api API, instanceID string, period int32) *Client {
	if period <= 0 {
		period = 300
	}
	return &Client{api: api, instanceID: instanceID, period: period}
}

func (c *Client) dim() []cwtypes.Dimension {
	return []cwtypes.Dimension{{Name: aws.String("InstanceId"), Value: aws.String(c.instanceID)}}
}

func (c *Client) query(id, metric, stat string) cwtypes.MetricDataQuery {
	return cwtypes.MetricDataQuery{
		Id: aws.String(id),
		MetricStat: &cwtypes.MetricStat{
			Metric: &cwtypes.Metric{
				Namespace:  aws.String("AWS/EC2"),
				MetricName: aws.String(metric),
				Dimensions: c.dim(),
			},
			Period: aws.Int32(c.period),
			Stat:   aws.String(stat),
		},
	}
}

// CPUAverages returns per-period Average CPUUtilization (percent) over the window.
func (c *Client) CPUAverages(ctx context.Context, start, end time.Time) ([]float64, error) {
	byTS, err := c.fetch(ctx, start, end, map[string]cwtypes.MetricDataQuery{
		"cpu": c.query("cpu", "CPUUtilization", "Average"),
	})
	if err != nil {
		return nil, err
	}
	out := make([]float64, 0, len(byTS["cpu"]))
	for _, v := range byTS["cpu"] {
		out = append(out, v)
	}
	return out, nil
}

// NetworkTotals returns per-period NetworkIn+NetworkOut (bytes), summed by
// timestamp so each value is the total traffic in that bucket.
func (c *Client) NetworkTotals(ctx context.Context, start, end time.Time) ([]float64, error) {
	byTS, err := c.fetch(ctx, start, end, map[string]cwtypes.MetricDataQuery{
		"in":  c.query("in", "NetworkIn", "Sum"),
		"out": c.query("out", "NetworkOut", "Sum"),
	})
	if err != nil {
		return nil, err
	}
	totals := make(map[time.Time]float64)
	for _, id := range []string{"in", "out"} {
		for ts, v := range byTS[id] {
			totals[ts] += v
		}
	}
	out := make([]float64, 0, len(totals))
	for _, v := range totals {
		out = append(out, v)
	}
	return out, nil
}

// fetch runs one GetMetricData call and returns, per query Id, a map of
// timestamp to value (so multiple metrics can be aligned by bucket).
func (c *Client) fetch(ctx context.Context, start, end time.Time, queries map[string]cwtypes.MetricDataQuery) (map[string]map[time.Time]float64, error) {
	in := &cloudwatch.GetMetricDataInput{
		StartTime: aws.Time(start),
		EndTime:   aws.Time(end),
		ScanBy:    cwtypes.ScanByTimestampDescending,
	}
	for _, q := range queries {
		in.MetricDataQueries = append(in.MetricDataQueries, q)
	}
	out, err := c.api.GetMetricData(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("get metric data: %w", err)
	}
	byID := make(map[string]map[time.Time]float64, len(queries))
	for _, r := range out.MetricDataResults {
		id := aws.ToString(r.Id)
		m := make(map[time.Time]float64, len(r.Values))
		for i, v := range r.Values {
			if i < len(r.Timestamps) {
				m[r.Timestamps[i]] = v
			}
		}
		byID[id] = m
	}
	return byID, nil
}
