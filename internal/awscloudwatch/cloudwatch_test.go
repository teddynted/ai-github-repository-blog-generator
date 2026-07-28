package awscloudwatch

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type fakeCW struct {
	results []cwtypes.MetricDataResult
}

func (f fakeCW) GetMetricData(_ context.Context, _ *cloudwatch.GetMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	return &cloudwatch.GetMetricDataOutput{MetricDataResults: f.results}, nil
}

func TestCPUAverages(t *testing.T) {
	t0 := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	c := New(fakeCW{results: []cwtypes.MetricDataResult{{
		Id:         aws.String("cpu"),
		Timestamps: []time.Time{t0, t0.Add(5 * time.Minute)},
		Values:     []float64{1.5, 2.0},
	}}}, "i-1", 300)
	got, err := c.CPUAverages(context.Background(), t0, t0.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	sort.Float64s(got)
	if len(got) != 2 || got[0] != 1.5 || got[1] != 2.0 {
		t.Errorf("cpu = %v, want [1.5 2]", got)
	}
}

func TestNetworkTotalsSumsInAndOutByTimestamp(t *testing.T) {
	t0 := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(5 * time.Minute)
	c := New(fakeCW{results: []cwtypes.MetricDataResult{
		{Id: aws.String("in"), Timestamps: []time.Time{t0, t1}, Values: []float64{100, 200}},
		{Id: aws.String("out"), Timestamps: []time.Time{t0, t1}, Values: []float64{10, 20}},
	}}, "i-1", 300)
	got, err := c.NetworkTotals(context.Background(), t0, t1)
	if err != nil {
		t.Fatal(err)
	}
	sort.Float64s(got)
	// bucket t0 = 100+10 = 110; bucket t1 = 200+20 = 220
	if len(got) != 2 || got[0] != 110 || got[1] != 220 {
		t.Errorf("network totals = %v, want [110 220]", got)
	}
}
