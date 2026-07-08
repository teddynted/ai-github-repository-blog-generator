# Monitoring & Observability

How the platform is observed in production: CloudWatch logs, metrics, dashboards, and alarms across the webhook front door, the SQS queue, the EC2 Spot host, and the n8n pipeline.

Related: [Infrastructure](./infrastructure.md) · [Security](./security.md) · [Monitoring Requirements](./requirements.md#8-non-functional-requirements).

---

## 1. Amazon CloudWatch

CloudWatch is the single pane of glass:

- **Logs** — log groups for `webhook-handler`, `idle-shutdown`, and the EC2 host (n8n), all with bounded retention (`LogRetentionDays`, default 14).
- **Metrics** — a custom `BlogGenerator` namespace for application metrics, plus native AWS metrics.
- **Dashboard** — one operational dashboard combining run health, latency, queue depth, and instance state.
- **Alarms** — threshold and anomaly alarms.

---

## 2. Metrics

### Application metrics (`BlogGenerator` namespace)

| Metric | Unit | Meaning |
| --- | --- | --- |
| `WebhookReceived` | Count | Webhook deliveries received |
| `WebhookRejected` | Count | Deliveries rejected (invalid signature) |
| `RunsStarted` | Count | Generation runs started (messages dequeued) |
| `RunsSucceeded` | Count | Runs that published content |
| `RunsFailed` | Count | Runs ending in failure |
| `RunDurationMs` | Milliseconds | End-to-end run latency (excludes cold start) |
| `ColdStartMs` | Milliseconds | Time from instance start to first message processed |
| `AssetsGenerated` | Count | Content assets produced per run |
| `InferenceLatencyMs` | Milliseconds | Local Ollama generation latency per content type |

### AWS-native metrics

| Source | Key metrics |
| --- | --- |
| API Gateway | `Count`, `4XXError`, `5XXError`, `Latency` |
| Lambda (`webhook-handler`, `idle-shutdown`) | `Invocations`, `Errors`, `Throttles`, `Duration` |
| SQS (`events`) | `ApproximateNumberOfMessagesVisible`, `ApproximateAgeOfOldestMessage` |
| SQS (`events-dlq`) | `ApproximateNumberOfMessagesVisible` (any message = attention) |
| EC2 | `CPUUtilization`, `StatusCheckFailed`, `GPUUtilization` (if published) |
| EBS | `VolumeReadOps`, `VolumeWriteOps`, `BurstBalance` |

---

## 3. Logging

- All components emit **structured JSON logs** (run ID, stage, status, duration).
- A **run ID** flows through every stage so a single generation can be traced from ingestion to publish across all n8n workflows.
- Logs **exclude secrets and raw source contents** ([Security §8](./security.md#8-logging--audit-trails)); they reference IDs and keys instead.

Example query (CloudWatch Logs Insights) — failures in the last hour:

```text
fields @timestamp, runId, stage, error
| filter status = "failed"
| sort @timestamp desc
| limit 50
```

---

## 4. Alerts

Alarms publish to an SNS topic subscribed by the operator (and optionally Slack).

| Alarm | Condition | Severity |
| --- | --- | --- |
| Webhook rejections | `WebhookRejected` spike (misconfig or attack) | Medium |
| Dead-letter queue not empty | `events-dlq` messages ≥ 1 | High |
| Queue backlog growing | `ApproximateAgeOfOldestMessage` > threshold | High |
| Run failure rate | `RunsFailed` ≥ 1 in 5 min (or failure ratio > 20%) | High |
| Handler Lambda errors | `webhook-handler` `Errors` > 0 | High |
| Idle-shutdown Lambda errors | `idle-shutdown` `Errors` > 0 (instance may not stop → cost) | Medium |
| API Gateway 5XX | `5XXError` > 0 | High |
| EC2 status check | `StatusCheckFailed` ≥ 1 | High |

---

## 5. Pipeline Failure Monitoring

- Each n8n workflow's error path emits a `RunsFailed` metric and logs a structured error with the run ID and failing stage ([Workflows §2](./workflows.md#2-end-to-end-pipeline)).
- The **Notifications** sub-workflow fires on failure, giving immediate human signal in addition to the CloudWatch alarm.
- Because runs are **idempotent** and messages return to SQS on failure, a failed run is safely retried (or lands in the DLQ) without corrupting output ([FR-5.5, FR-5.6](./requirements.md#15-reliability-retry-logging--error-handling)).

---

## 6. Cold Start & Spot Interruption Monitoring

| Signal | Meaning | Response |
| --- | --- | --- |
| Elevated `ColdStartMs` | Instance/model warm-up is slow | Verify EBS re-attach; confirm model is cached, not re-downloaded |
| `StatusCheckFailed` / instance terminated | Possible Spot interruption | SQS visibility timeout returns the message; a new instance resumes |
| DLQ messages appearing | Repeated processing failure | Inspect payload and n8n logs; fix and redrive |

Spot interruptions are expected and handled: in-flight work returns to the queue and is retried on the next instance ([Cost Optimisation §3](./cost-optimization.md#3-ec2-spot-instances)).

---

## 7. Cost & Idle Monitoring

- Track instance **running hours** (via EC2 state transitions) to confirm the idle-shutdown is working — the instance should be `stopped` when idle.
- An `idle-shutdown` Lambda error is treated as a **cost risk**: if the instance fails to stop, it keeps billing.
- Because inference is local, there are **no token/usage metrics to bill** — cost tracking focuses on EC2 running time and EBS size ([Cost Optimisation](./cost-optimization.md)).

---

## 8. Operational Runbook (quick reference)

| Situation | First checks |
| --- | --- |
| Run failed | Logs Insights query (§3) by run ID → identify failing stage |
| No output appearing | Confirm the webhook delivered (GitHub Recent Deliveries); check SQS depth; confirm EC2 started |
| Messages stuck in queue | Is the instance running? Check n8n polling; inspect DLQ |
| Instance won't stop | Check `idle-shutdown` Lambda logs and EventBridge rule |
| High cost | Confirm instance stops when idle; review EBS size and log retention |
