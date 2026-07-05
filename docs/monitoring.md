# Monitoring & Observability

How the platform is observed in production: CloudWatch logs, metrics, dashboards, and alarms across infrastructure, workflows, Amazon Bedrock, and the application.

Related: [Infrastructure](./infrastructure.md) · [Security](./security.md) · [Requirements → Observability](./requirements.md#29-observability).

---

## 1. Amazon CloudWatch

CloudWatch is the single pane of glass:

- **Logs** — one log group per Lambda plus n8n logs, all with bounded retention (`log_retention_days`, default 14).
- **Metrics** — a custom `BlogGenerator` namespace for application metrics, plus native AWS metrics.
- **Dashboard** — one operational dashboard combining run health, latency, errors, and cost signals.
- **Alarms** — threshold and anomaly alarms wired to SNS.

---

## 2. Metrics

### Application metrics (`BlogGenerator` namespace)

| Metric | Unit | Meaning |
| --- | --- | --- |
| `RunsStarted` | Count | Generation runs triggered |
| `RunsSucceeded` | Count | Runs producing a published post |
| `RunsFailed` | Count | Runs ending in failure |
| `RunDurationMs` | Milliseconds | End-to-end run latency |
| `BedrockInputTokens` | Count | Prompt tokens per run |
| `BedrockOutputTokens` | Count | Generated tokens per run |
| `BedrockLatencyMs` | Milliseconds | Model invocation latency |

### AWS-native metrics

| Source | Key metrics |
| --- | --- |
| Lambda | `Invocations`, `Errors`, `Throttles`, `Duration`, `ConcurrentExecutions` |
| EC2 | `CPUUtilization`, `StatusCheckFailed` |
| S3 | `4xxErrors`, `5xxErrors`, `BucketSizeBytes` |
| SNS | `NumberOfNotificationsFailed` |

---

## 3. Logging

- All components emit **structured JSON logs** (correlation/run ID, stage, status, duration).
- A **run ID** flows through every stage so a single blog generation can be traced across n8n and all three Lambdas.
- Logs **exclude secrets and raw source contents** ([Security §7](./security.md#7-logging--audit-trails)); they reference S3 keys and IDs instead.

Example query (CloudWatch Logs Insights) — failures in the last hour:

```text
fields @timestamp, runId, stage, error
| filter status = "failed"
| sort @timestamp desc
| limit 50
```

---

## 4. Alerts

Alarms publish to an SNS topic subscribed by the operator (`notification_email`) and optionally Slack.

| Alarm | Condition | Severity |
| --- | --- | --- |
| Run failure rate | `RunsFailed` ≥ 1 in 5 min (or failure ratio > 20%) | High |
| Lambda errors | Any function `Errors` > 0 sustained | High |
| Lambda throttles | `Throttles` > 0 | Medium |
| Bedrock failures | Invocation error/anomaly (see §6) | High |
| EC2 status check | `StatusCheckFailed` ≥ 1 | High |
| SNS delivery | `NumberOfNotificationsFailed` > 0 | Medium |
| Cost/token anomaly | `BedrockOutputTokens` anomaly | Low |

---

## 5. Workflow Failure Monitoring

- Each n8n workflow's error path emits a `RunsFailed` metric and logs a structured error with the run ID and failing stage ([Workflows §2](./workflows.md#2-end-to-end-pipeline)).
- The **Notifications** sub-workflow fires on failure, giving immediate human signal in addition to the CloudWatch alarm.
- Because runs are idempotent and non-destructive ([FR-16.3](./requirements.md#116-error-handling)), a failed run can be safely retried once the root cause is fixed.

---

## 6. Amazon Bedrock Failure Monitoring

| Failure | Signal | Response |
| --- | --- | --- |
| `ThrottlingException` | Elevated `BedrockLatencyMs`, retry logs | Automatic backoff/retry; request quota increase if persistent |
| `AccessDeniedException` | Errors on invoke | Verify model access + IAM scope ([Deployment §1](./deployment.md#1-aws-prerequisites)) |
| `ValidationException` | Invoke errors | Check model ID/params and prompt size |
| Elevated token usage | `BedrockOutputTokens` anomaly alarm | Review prompt template / truncation |

Token metrics feed cost tracking in [Cost Optimization](./cost-optimization.md).

---

## 7. Infrastructure Monitoring

- **EC2:** CPU and status-check alarms; the instance is otherwise mostly stopped by schedule.
- **Lambda:** error/throttle/duration alarms per function.
- **S3:** 5xx and size metrics; lifecycle keeps storage bounded.
- **Networking:** VPC Flow Logs (optional) for traffic auditing.

---

## 8. Application Monitoring

- The dashboard's top row shows **RunsStarted / Succeeded / Failed** and **RunDurationMs (p50/p90)** for at-a-glance health.
- Token metrics and per-stage latency help diagnose slow or expensive runs.
- Every generated post carries front-matter metadata (`model`, `date`, `source_repo`) so outputs are traceable back to the run that produced them.

---

## 9. Operational Runbook (quick reference)

| Situation | First checks |
| --- | --- |
| Run failed | Logs Insights query (§3) by run ID → identify failing stage |
| No posts appearing | Confirm schedule fired; check ingestion logs; verify posts-bucket write perms |
| Bedrock errors | Model access + IAM + Region; check throttling |
| High cost | Token metrics; artifact/log retention; EC2 stopped as scheduled |
