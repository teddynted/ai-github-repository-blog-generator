# Widget v1.0.0 — first production release

First production release of **Widget**, an event-driven serverless widget-processing platform on AWS.

## Features

- **EventBridge ingestion pipeline** — inbound events are validated and published to Amazon EventBridge.
- **SQS durable buffer** — a rule routes events into an Amazon SQS queue, with a dead-letter queue for poison messages.
- **Hardened EC2 worker** — a scheduled EC2 worker drains the queue and processes each widget, with IMDSv2 enforced and audit logging enabled.
- **CloudFormation infrastructure** — the EventBridge bus, SQS queue + DLQ, and EC2 worker are defined as one stack.

## Bug Fixes

- Fixed a race condition in the SQS drainer.

## Breaking Changes

- The legacy polling endpoint has been **removed**. Migrate to the webhook + EventBridge ingestion path.
