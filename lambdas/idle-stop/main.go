// Command idle-stop is the AWS Lambda invoked on a schedule (hourly by default)
// by an EventBridge rule. It stops the on-demand EC2 host (INSTANCE_ID) only
// when it has been idle — CPU and network below threshold AND no active n8n work — for a sustained IDLE_MINUTES. Idleness is tracked across
// invocations by an IdleSince tag on the instance, so the Lambda stays
// stateless. It is independent of the start path and honours a KEEP_RUNNING tag
// override.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/awscloudwatch"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/awsec2"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/idle"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/idleprobe"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/power"
)

const (
	idleSinceTag   = "IdleSince"
	keepRunningTag = "KEEP_RUNNING"
)

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if err := a.Config.Require("AWSRegion", "InstanceID"); err != nil {
		log.Fatalf("config: %v", err)
	}
	instanceID := a.Config.InstanceID

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(a.Config.AWSRegion),
		awsconfig.WithRetryMode(aws.RetryModeAdaptive),
		awsconfig.WithRetryMaxAttempts(5),
	)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	ec2Client := ec2.NewFromConfig(awsCfg)
	instances := awsec2.NewIdle(ec2Client)
	sw := &power.Switch{Instances: awsec2.New(ec2Client), InstanceID: instanceID, Logger: a.Logger}
	metrics := awscloudwatch.New(cloudwatch.NewFromConfig(awsCfg), instanceID, 300)

	probeTimeout := envDuration("PROBE_TIMEOUT", 5*time.Second)
	var probes []idle.Probe
	if u := os.Getenv("N8N_URL"); u != "" {
		probes = append(probes, idleprobe.N8N(u, os.Getenv("N8N_API_KEY"), probeTimeout, a.Logger))
	}
	ev := &idle.Evaluator{
		Cfg: idle.Config{
			CPUThreshold: envFloat("CPU_THRESHOLD", 5),
			NetThreshold: envFloat("NET_THRESHOLD_BYTES", 1<<20),
			IdleDuration: time.Duration(envInt("IDLE_MINUTES", 30)) * time.Minute,
			StartupGrace: time.Duration(envInt("STARTUP_GRACE_MINUTES", 15)) * time.Minute,
			// Default trailing window matches the hourly cadence, so each run
			// inspects the whole preceding hour rather than a 10-minute sliver.
			EvalWindow: time.Duration(envInt("EVAL_WINDOW_MINUTES", 60)) * time.Minute,
		},
		Metrics: metrics,
		Probes:  probes,
	}

	snsTopic := os.Getenv("SNS_TOPIC_ARN")
	dryRun := envBool("DRY_RUN", false)
	var notifier *sns.Client
	if snsTopic != "" {
		notifier = sns.NewFromConfig(awsCfg)
	}

	lambda.Start(func(ctx context.Context, _ json.RawMessage) (string, error) {
		snap, err := instances.Snapshot(ctx, instanceID)
		if err != nil {
			a.Logger.Error("snapshot failed", "error", err.Error())
			return "", err
		}
		if snap.State == "" {
			a.Logger.Error("instance not found", "instance_id", instanceID)
			return "skip", nil
		}

		inst := idle.Instance{
			State:       snap.State,
			LaunchTime:  snap.LaunchTime,
			KeepRunning: snap.Tags[keepRunningTag] == "true",
		}
		if ts := snap.Tags[idleSinceTag]; ts != "" {
			if parsed, perr := time.Parse(time.RFC3339, ts); perr == nil {
				inst.IdleSince = parsed
			}
		}

		d, sig, err := ev.Evaluate(ctx, inst)
		if err != nil {
			// A metrics failure must never stop the instance blind.
			a.Logger.Error("evaluation failed; keeping instance running", "error", err.Error())
			return "", err
		}
		a.Logger.Info("idle evaluation",
			"instance_id", instanceID, "action", string(d.Action), "reason", d.Reason,
			"idle_for_min", d.IdleFor.Minutes(), "cpu_idle", sig.CPUIdle, "net_idle", sig.NetIdle,
			"busy_probe", sig.BusyProbe, "state", snap.State)

		if d.MarkIdle {
			if err := instances.SetTag(ctx, instanceID, idleSinceTag, time.Now().UTC().Format(time.RFC3339)); err != nil {
				a.Logger.Warn("set IdleSince tag failed", "error", err.Error())
			}
		}
		if d.ClearIdle {
			if err := instances.DeleteTag(ctx, instanceID, idleSinceTag); err != nil {
				a.Logger.Warn("delete IdleSince tag failed", "error", err.Error())
			}
		}

		if d.Action != idle.ActionStop {
			return string(d.Action), nil
		}

		msg := "Stopping " + instanceID + ": idle for " + strconv.Itoa(int(d.IdleFor.Minutes())) + "m (CPU/network below threshold, no n8n activity)."
		if dryRun {
			a.Logger.Info("DRY_RUN: would stop", "instance_id", instanceID, "message", msg)
			return "dry_run_stop", nil
		}
		notify(ctx, notifier, snsTopic, msg, a.Logger)
		if _, err := sw.EnsureStopped(ctx); err != nil {
			a.Logger.Error("stop failed", "error", err.Error())
			return "", err
		}
		a.Logger.Info("instance stopped", "instance_id", instanceID, "idle_for_min", d.IdleFor.Minutes())
		return "stop", nil
	})
}

func notify(ctx context.Context, client *sns.Client, topic, msg string, logger interface{ Warn(string, ...any) }) {
	if client == nil || topic == "" {
		return
	}
	_, err := client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(topic),
		Subject:  aws.String("EC2 idle auto-stop"),
		Message:  aws.String(msg),
	})
	if err != nil {
		logger.Warn("sns publish failed", "error", err.Error())
	}
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
