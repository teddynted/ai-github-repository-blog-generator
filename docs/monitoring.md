# Monitoring & Observability

How the platform is observed in production: CloudWatch logs, metrics, dashboards, and alarms across infrastructure, workflows, Amazon Bedrock, and the application.

Related: [Infrastructure](./infrastructure.md) · [Security](./security.md) · [Monitoring Requirements](./requirements.md#10-monitoring-requirements).

---

## 1. Amazon CloudWatch

CloudWatch is the single pane of glass:

- **Logs** — log groups for n8n and the `ec2-scheduler` Lambda, all with bounded retention (`log_retention_days`, default 14).
- **Metrics** — a custom `BlogGenerator` namespace for application metrics, plus native AWS metrics.
- **Dashboard** — one operational dashboard combining run health, latency, errors, and cost signals.
- **Alarms** — threshold and anomaly alarms wired to SNS.

---

## 2. Metrics

### Application metrics (`BlogGenerator` namespace)

| Metric | Unit | Meaning |
| --- | --- | --- |
| `RegistrationsCompleted` | Count | Repositories registered successfully |
| `RegistrationsFailed` | Count | Registration attempts that failed |
| `WebhookReceived` | Count | Webhook deliveries received |
| `WebhookRejected` | Count | Deliveries rejected (invalid signature) |
| `RunsStarted` | Count | Generation runs triggered (valid webhooks) |
| `RunsSucceeded` | Count | Runs producing a stored content package |
| `RunsFailed` | Count | Runs ending in failure |
| `RunDurationMs` | Milliseconds | End-to-end run latency |
| `AssetsGenerated` | Count | Content assets produced per run |
| `BedrockInputTokens` | Count | Prompt tokens per run |
| `BedrockOutputTokens` | Count | Generated tokens per run |
| `BedrockLatencyMs` | Milliseconds | Model invocation latency |

### AWS-native metrics

| Source | Key metrics |
| --- | --- |
| Lambda (`ec2-scheduler`) | `Invocations`, `Errors`, `Throttles`, `Duration` |
| EC2 | `CPUUtilization`, `StatusCheckFailed` |
| S3 | `4xxErrors`, `5xxErrors`, `BucketSizeBytes` |
| SNS | `NumberOfNotificationsFailed` |

---

## 3. Logging

- All components emit **structured JSON logs** (correlation/run ID, stage, status, duration).
- A **run ID** flows through every stage so a single content-package generation can be traced across all n8n workflows.
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
| Registration failures | `RegistrationsFailed` > 0 sustained | Medium |
| Webhook rejections | `WebhookRejected` spike (possible misconfig or attack) | Medium |
| Run failure rate | `RunsFailed` ≥ 1 in 5 min (or failure ratio > 20%) | High |
| Scheduler Lambda errors | `ec2-scheduler` `Errors` > 0 | High |
| Bedrock failures | Invocation error/anomaly (see §6) | High |
| EC2 status check | `StatusCheckFailed` ≥ 1 | High |
| SNS delivery | `NumberOfNotificationsFailed` > 0 | Medium |
| Cost/token anomaly | `BedrockOutputTokens` anomaly | Low |

---

## 5. Workflow Failure Monitoring

- Each n8n workflow's error path emits a `RunsFailed` metric and logs a structured error with the run ID and failing stage ([Workflows §2](./workflows.md#2-end-to-end-pipeline)).
- The **Notifications** sub-workflow fires on failure, giving immediate human signal in addition to the CloudWatch alarm.
- Because runs are idempotent and non-destructive ([FR-6.3](./requirements.md#16-error-handling--retries)), a failed run can be safely retried once the root cause is fixed.

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

- **EC2:** CPU and status-check alarms; the instance is otherwise stopped outside the 19:00–21:00 window.
- **Lambda:** error/duration alarms on `ec2-scheduler`.
- **S3:** 5xx and size metrics; lifecycle keeps storage bounded.
- **Networking:** VPC Flow Logs (optional) for traffic auditing.

---

## 8. Application Monitoring

- The dashboard's top row shows **RunsStarted / Succeeded / Failed** and **RunDurationMs (p50/p90)** for at-a-glance health.
- Token metrics and per-stage latency help diagnose slow or expensive runs.
- Every generated package carries `metadata.json` (`model`, `generated_at`, `source_repository`, `reading_time_minutes`, `tags`) so outputs are traceable back to the run that produced them.

---

## 9. Operational Runbook (quick reference)

| Situation | First checks |
| --- | --- |
| Run failed | Logs Insights query (§3) by run ID → identify failing stage |
| No packages appearing | Confirm the trigger fired and EC2 is running; check ingestion logs; verify generated-content bucket write perms |
| Bedrock errors | Model access + IAM + Region; check throttling |
| High cost | Token metrics; content/log retention; EC2 stopped as scheduled |
