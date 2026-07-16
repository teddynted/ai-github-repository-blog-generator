# Monitoring & Observability

How the platform is observed in production: CloudWatch logs, metrics, dashboards, and alarms across the webhook front door, the EventBridge/SQS path, the scheduled On-Demand host, and the n8n pipeline. A key signal is the **ratio of triggered to ignored events** — most webhooks should be acknowledged and ignored.

Related: [Infrastructure](./infrastructure.md) · [Security](./security.md) · [Monitoring Requirements](./requirements.md#11-non-functional-requirements).

---

## 1. Amazon CloudWatch

CloudWatch is the single pane of glass:

- **Logs** — log groups for `registration`, `webhook-handler`, `manual-trigger`, `scheduled-start`, `scheduled-stop`, and the EC2 host (n8n), with bounded retention (`LogRetentionDays`, default 14).
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
| `WebhookReceived` | Count | Webhook deliveries received |
| `WebhookRejected` | Count | Deliveries rejected (invalid signature) |
| `TriggerMatched` | Count | Deliveries whose commit message matched the publish trigger |
| `WebhookIgnored` | Count | Valid deliveries acknowledged and ignored (no trigger match) |
| `RunsStarted` | Count | Runs started (matched events dequeued) |
| `RunsSucceeded` | Count | Runs that published content |
| `RunsFailed` | Count | Runs ending in failure |
| `RunDurationMs` | Milliseconds | End-to-end run latency (excludes cold start) |
| `ColdStartMs` | Milliseconds | Time from instance start to first message processed |
| `ApprovalsPending` | Count | Runs waiting on human approval |
| `AssetsGenerated` | Count | Content assets produced per run |
| `InferenceLatencyMs` | Milliseconds | Local Ollama generation latency per content type |

> A healthy system typically shows **`WebhookIgnored` ≫ `TriggerMatched`** — most commits are routine and correctly filtered out.

### AWS-native metrics

| Source | Key metrics |
| --- | --- |
| API Gateway | `Count`, `4XXError`, `5XXError`, `Latency` |
| Lambda (`registration`, `webhook-handler`, `manual-trigger`, `scheduled-start`, `scheduled-stop`) | `Invocations`, `Errors`, `Throttles`, `Duration` |
| DynamoDB (`repositories`) | `ThrottledRequests`, `System/UserErrors` |
| Secrets Manager | `GetSecretValue` call volume (audit spikes) |
| EventBridge | `Invocations`, `FailedInvocations`, `ThrottledRules` |
| SQS (`events`) | `ApproximateNumberOfMessagesVisible`, `ApproximateAgeOfOldestMessage` |
| SQS (`events-dlq`) | `ApproximateNumberOfMessagesVisible` (any message = attention) |
| EC2 | `CPUUtilization`, `StatusCheckFailed`, `GPUUtilization` (if published) |
| EBS | `VolumeReadOps`, `VolumeWriteOps`, `BurstBalance` |

---

## 3. Logging

- All components emit **structured JSON logs** (run ID, stage, status, duration).
- A **run ID** flows through every stage so a single generation can be traced from the matched event to publish across all n8n workflows.
- The handler logs each delivery's **outcome** (`published`, `ignored`, `rejected`) with the delivery ID — never the raw payload or secret.
- Logs **exclude secrets and raw source contents** ([Security §8](./security.md#9-logging--audit-trails)).

Example query (CloudWatch Logs Insights) — ignored vs published in the last day:

```text
fields @timestamp, deliveryId, outcome
| filter outcome in ["published", "ignored", "rejected"]
| stats count() by outcome
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
| Manual trigger errors | `manual-trigger` `Errors` > 0 (POST /process failing) | Medium |
| Scheduled-start errors | `scheduled-start` `Errors` > 0 (host may not come up for its window) | High |
| Scheduled-stop errors | `scheduled-stop` `Errors` > 0 (instance may not stop at 20:00 → cost) | High |
| EventBridge failures | `FailedInvocations` > 0 | High |
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
| No start at 18:00 on a weekday | `scheduled-start` failed or schedule misconfigured | Check the start Lambda logs and `ScheduleTimezone`/`ScheduleState` |
| DLQ messages appearing | Repeated processing failure | Inspect payload and n8n logs; fix and redrive |

Because the instance stops and starts on the schedule, in-flight work at 20:00 that does not complete returns to the queue and is retried at the next start ([Cost Optimisation §5](./cost-optimization.md#5-on-demand-on-a-schedule-not-spot)).

---

## 7. Cost & Idle Monitoring

- Track **triggered vs ignored** ratio — a sudden rise in `TriggerMatched` may indicate misuse of the `blog:` trigger and rising cost.
- Track instance **running hours** to confirm the schedule works — the instance should be `running` only 18:00–20:00 on weekdays and `stopped` otherwise.
- A `scheduled-stop` error is a **cost risk**: if the instance fails to stop at 20:00, it keeps billing until the next successful stop.
- Because inference is local, there are **no token/usage metrics to bill** — cost tracking focuses on EC2 running time and EBS size ([Cost Optimisation](./cost-optimization.md)).

---

## 8. Operational Runbook (quick reference)

| Situation | First checks |
| --- | --- |
| A `blog:` commit produced nothing | Confirm delivery (GitHub Recent Deliveries); check handler logged `published` (and `accepted`/`deferred`); check EventBridge/SQS; confirm the instance is up (in-window) or will process at the next 18:00 start |
| A normal commit produced content | Check the configured `PublishTrigger`; review handler trigger logic |
| Messages stuck in queue | Is it outside the window (expected — drains at next start)? In-window: check n8n polling; inspect DLQ |
| Instance won't stop | Check `scheduled-stop` logs and the EventBridge stop schedule (`ScheduleState`, timezone) |
| High cost | Confirm the instance is stopped outside the window; check triggered/ignored ratio; review EBS size and log retention |
