# Changelog

All notable changes to Widget are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/) and the project uses
[Semantic Versioning](https://semver.org/).

## [1.0.0] - 2026-01-01

### Added
- EventBridge ingestion pipeline for inbound widget events.
- Amazon SQS durable buffer with a dead-letter queue.
- Hardened EC2 worker (IMDSv2, auditd, least-privilege IAM).
- CloudFormation stack defining the EventBridge bus, SQS queue + DLQ, and EC2 worker.

### Fixed
- Race condition in the SQS drainer.

### Removed
- **BREAKING:** the legacy polling endpoint. Use the webhook + EventBridge ingestion path instead.

## [0.9.0] - 2025-12-01

### Added
- Initial widget-processing prototype (polling-based).
