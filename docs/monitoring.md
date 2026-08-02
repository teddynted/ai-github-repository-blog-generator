# Monitoring & Observability

How the platform is observed in production: CloudWatch logs, metrics, dashboards, and alarms across the `/process` front door, the Step Functions / SQS path, the on-demand host, and the worker + n8n pipeline. Key signals are **run success rate**, **queue age**, and the **Provider Router fallback rate**.

Related: [Infrastructure](./infrastructure.md) · [Security](./security.md) · [Monitoring Requirements](./requirements.md#11-non-functional-requirements).

---

## 1. Amazon CloudWatch

CloudWatch is the single pane of glass:

- **Logs** — log groups for `registration`, `manual-trigger`, `release-context`, `scheduled-start`, `scheduled-stop`, and the EC2 host (worker + n8n), with bounded retention (`LogRetentionDays`, default 14).
- **Metrics** — a custom `BlogGenerator` namespace, plus native AWS metrics.
- **Dashboard** — one operational dashboard combining trigger activity, run health, latency, queue depth, and instance state.
- **Alarms** — threshold and anomaly alarms.

---

## 2. Metrics

### Application metrics (`BlogGenerator` namespace)

| Metric | Unit | Meaning |
| --- | --- | --- |
| `RegistrationsCompleted` | Count | Repositories registered successfully |
| `RegistrationsFailed` | Count | Registration attempts that failed (access/permission validation) |
| `ProcessRequested` | Count | `POST /process` requests accepted (executions started) |
| `RunsStarted` | Count | Runs started (jobs dequeued) |
| `RunsSucceeded` | Count | Runs that published content |
| `RunsFailed` | Count | Runs ending in failure |
| `ArtifactsReused` | Count | Artifacts skipped because they already exist in S3 (idempotency) |
| `ProviderFallbacks` | Count | Generations that fell back from Bedrock to the Anthropic API |
| `RunDurationMs` | Milliseconds | End-to-end run latency (excludes cold start) |
| `ColdStartMs` | Milliseconds | Time from instance start to first message processed |
| `ApprovalsPending` | Count | Runs waiting on human approval |
| `AssetsGenerated` | Count | Content assets produced per run |
| `InferenceLatencyMs` | Milliseconds | Provider Router generation latency per content type |

> A rising **`ProviderFallbacks`** rate means Bedrock is throttling — check Bedrock quotas; the Anthropic fallback keeps runs succeeding meanwhile.

### AWS-native metrics

| Source | Key metrics |
| --- | --- |
| API Gateway | `Count`, `4XXError`, `5XXError`, `Latency` |
| Lambda (`registration`, `manual-trigger`, `release-context`, `scheduled-start`, `scheduled-stop`) | `Invocations`, `Errors`, `Throttles`, `Duration` |
| Step Functions | `ExecutionsStarted`, `ExecutionsFailed`, `ExecutionsTimedOut`, `ExecutionTime` |
| DynamoDB (`repositories`) | `ThrottledRequests`, `System/UserErrors` |
| Secrets Manager | `GetSecretValue` call volume (audit spikes) |
| SQS (`events`) | `ApproximateNumberOfMessagesVisible`, `ApproximateAgeOfOldestMessage` |
| SQS (`events-dlq`) | `ApproximateNumberOfMessagesVisible` (any message = attention) |
| EC2 | `CPUUtilization`, `StatusCheckFailed` |
| EBS | `VolumeReadOps`, `VolumeWriteOps`, `BurstBalance` |

---

## 3. Logging

- All components emit **structured JSON logs** (run ID, stage, status, duration).
- A **run ID** flows through every stage so a single generation can be traced from the `/process` request through generation to publish.
- The worker logs each run's **outcome** (`published`, `reused`, `failed`) with the run ID — never the raw payload or secret.
- Logs **exclude secrets and raw source contents** ([Security §8](./security.md#9-logging--audit-trails)).

Example query (CloudWatch Logs Insights) — run outcomes in the last day:

```text
fields @timestamp, runId, outcome
| filter outcome in ["published", "reused", "failed"]
| stats count() by outcome
```

### Worker instance logs (`/blog-gen/instance`)

The On-Demand worker host has **no SSH and no SSM shell** (least privilege), so it
is diagnosed entirely through CloudWatch. The instance runs the CloudWatch agent
(configured in the compute stack UserData) and ships to the `/blog-gen/instance`
log group, one stream per source, keyed by instance id:

| Stream | Source | Use |
| --- | --- | --- |
| `{id}/boot` | `/var/log/cloud-init-output.log` | Boot + provisioning (Docker install, compose up, service start) |
| `{id}/syslog` | `/var/log/syslog` | Full systemd/journald |
| `{id}/worker` | `blog-gen-worker` | The pipeline: clone/context → generate (Bedrock → Anthropic) → S3 → review → publish |
| `{id}/health` | 60s `health.sh` probe | `READY` / `UNHEALTHY: <reason>` |

**Watch the box live** (the fastest way to see a run happen or a failure land):

```bash
aws logs tail /blog-gen/instance --follow --region us-east-1
```

Useful variants:

```bash
# include more boot history, then follow
aws logs tail /blog-gen/instance --follow --since 30m --region us-east-1

# only the worker + errors
aws logs tail /blog-gen/instance --follow --region us-east-1 \
  --filter-pattern '?worker ?error ?panic ?failed'

# is the box actually ready? (health probe)
aws logs tail /blog-gen/instance --since 5m --region us-east-1 \
  --log-stream-name-prefix "$(aws ec2 describe-instances \
    --filters Name=tag:Project,Values=blog-gen Name=instance-state-name,Values=running \
    --query 'Reservations[0].Instances[0].InstanceId' --output text)/health"
```

> There is a few-second ingestion delay (the agent batches), so lines appear
> shortly after they're written on the host — normal, not a stall.

### Instance resource metrics (`BlogGen/Instance` namespace)

The same agent emits **CPU / memory / disk** (on `/` and `/data`) so the failure
modes logs don't show are visible: **disk-full**, **OOM** (a 7B model on a
memory-tight box), and **CPU pinning** (explains slow generation). Pair these with
the `{id}/worker` stream when a run is slow or dies without a clean error.

---

## 4. Alerts

Alarms publish to an SNS topic subscribed by the operator (and optionally Slack).

| Alarm | Condition | Severity |
| --- | --- | --- |
| Provider fallback spike | `ProviderFallbacks` rising (Bedrock throttling) | Medium |
| Dead-letter queue not empty | `events-dlq` messages ≥ 1 | High |
| Queue backlog growing | `ApproximateAgeOfOldestMessage` > threshold | High |
| Run failure rate | `RunsFailed` ≥ 1 in 5 min (or failure ratio > 20%) | High |
| Manual trigger errors | `manual-trigger` `Errors` > 0 (POST /process failing) | High |
| State machine failures | Step Functions `ExecutionsFailed`/`ExecutionsTimedOut` > 0 | High |
| Scheduled-start errors | `scheduled-start` `Errors` > 0 (host may not come up for its window) | High |
| Scheduled-stop errors | `scheduled-stop` `Errors` > 0 (instance may not stop → cost) | High |
| API Gateway 5XX | `5XXError` > 0 | High |
| EC2 status check | `StatusCheckFailed` ≥ 1 | High |
| Approval backlog | `ApprovalsPending` sustained | Low |

---

## 5. Pipeline Failure Monitoring

- Each n8n workflow's error path emits a `RunsFailed` metric and logs a structured error with the run ID and failing stage ([Workflows §3](./workflows.md#3-end-to-end-pipeline)).
- The **Notifications** sub-workflow fires on failure and on approval requests, giving immediate human signal.
- Because runs are **idempotent** and messages return to SQS on failure, a failed run is safely retried (or lands in the DLQ) without corrupting output ([FR-6.5, FR-6.6](./requirements.md#16-reliability-retry-logging--error-handling)).

---

## 6. Startup & Schedule Monitoring

| Signal | Meaning | Response |
| --- | --- | --- |
| Elevated `ColdStartMs` | Instance/model warm-up is slow at the 18:00 start | Verify EBS re-attach; confirm model is cached, not re-downloaded |
| `StatusCheckFailed` / instance terminated | Host unhealthy or replaced | SQS visibility timeout returns the message; the next scheduled start resumes work |
| No start at 18:00 | `scheduled-start` failed or schedule misconfigured | Check the start Lambda logs and `ScheduleTimezone`/`ScheduleState` |
| DLQ messages appearing | Repeated processing failure | Inspect payload and n8n logs; fix and redrive |

Because the instance stops and starts on the schedule, in-flight work at 20:00 that does not complete returns to the queue and is retried at the next start ([Cost Optimisation §5](./cost-optimization.md#5-on-demand-on-a-schedule-not-spot)).

---

## 7. Cost & Idle Monitoring

- Track **`ArtifactsReused`** — a high reuse ratio confirms idempotency is saving inference cost; a sudden drop means new work is being generated.
- Track instance **running hours** to confirm the host stops after runs — it should be `stopped` outside the window / after idle-stop.
- A `scheduled-stop` error is a **cost risk**: if the instance fails to stop, it keeps billing until the next successful stop.
- Inference is **per-token on Bedrock/Anthropic** — watch `ProviderFallbacks` and Bedrock usage in Cost Explorer alongside EC2 running time and EBS size ([Cost Optimisation](./cost-optimization.md)).

---

## 8. Operational Runbook (quick reference)

| Situation | First checks |
| --- | --- |
| A `/process` call produced nothing | Check the Step Functions execution (did it reach EnqueueJob?); confirm the instance started and reported SSM `Online`; check the worker log for the run id |
| A run failed at generation | Check the worker log for `ai provider failed`; verify Bedrock model access or the Anthropic key secret; the router logs which leg it used |
| Messages stuck in queue | Is the worker up (instance running)? Check the worker is draining; inspect DLQ |
| Instance won't stop | Check `scheduled-stop` logs and the EventBridge stop schedule (`ScheduleState`, timezone), and idle-stop settings |
| High cost | Confirm the instance is stopped after runs; check `ArtifactsReused` (idempotency working?) and Bedrock token usage; review EBS size and log retention |
