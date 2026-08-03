# Phase B Teardown: retire the EC2 worker path after the serverless cutover

**Status:** prepared, NOT executed · **Prereq:** `CUTOVER_CONTENT=true` live + a soak period on serverless · **Date:** 2026-08-03

Once `POST /process` has run cleanly on the serverless `blog-gen-content` pipeline for a
soak period, this retires the now-dormant EC2 release path and shrinks the box. **Do not
start until you're willing to give up the instant `CUTOVER_CONTENT` rollback** — this
removes it.

## ⚠️ Assumption to confirm first — snapshot runs

The serverless path is **release-only** (`content-runner` requires `RELEASE_TAG`). The EC2
worker also handled **snapshot/repository runs** (`POST /process` *without* `releaseTag`).
All observed usage is release runs, so this teardown assumes snapshot runs are unused. **If
you ever call `/process` without a `releaseTag`, do NOT remove the worker** — that path has
no serverless equivalent yet. Confirm before executing.

## Cross-stack ordering (critical)

`compute.yaml` imports the SQS exports that `serverless.yaml` publishes:
`compute.yaml:190` (`QueueArn`), `572`/`636` (`QueueUrl`). CloudFormation **refuses to delete
an export that another stack imports.** So the teardown must deploy in this order:

1. **compute first** — remove the worker + its `QueueArn`/`QueueUrl` imports (stops importing).
2. **serverless second** — remove the orchestration SM + SQS queues + their exports (now unimported).

Reversing this fails with `Export ... cannot be deleted as it is in use by blog-gen-compute`.

## Changes, file by file

### 1. `cmd/worker/main.go` — remove the release path (code)
- Delete the release-pipeline construction (`main()` lines ~188–245: `releaseSrc`, `routedKinds`,
  `provenance`, `releasePipe`, the `ArtifactStore` closure) and the now-unused locals
  `analysisModel`, `provName`, `provModel` (~170–171).
- Delete the video-trigger wiring (~275–288: `var relRunner …` + the `videoTriggeringRunner`
  block) and drop `relRunner` from the `run(…)` call (~291).
- Delete the `releaseRunner` + `sfnStarter` interfaces, the `videoTriggeringRunner` struct +
  `Run` method (~306–356), and `handleReleaseMessage` (~426–454).
- Drop the `rr releaseRunner` parameter from `run(…)` and `handleMessage(…)`, and delete the
  release branch in `handleMessage` (~392–395).
- Remove now-unused imports: `aws`, `service/sfn`, `contentsuite`, `engineeringanalysis`,
  `releasepipeline`, `releasesource`, `releasecontext (rc)`, `promptversion`, `artifactstore`,
  `github`. Keep `airouter`, `releasegen`, `reposource`, `metadata`, `secrets` (snapshot path).
- Delete `cmd/worker/video_test.go` (tests the removed decorator).
- **If snapshot runs are also unused (see assumption): delete `cmd/worker/` entirely instead**,
  and remove the worker build/upload from `deploy.yml` (lines 85–86, 110) + the worker service
  from the compute userdata (below).
- Verify: `go build ./... && go vet ./... && go test ./...`.

### 2. `infrastructure/serverless.yaml` — remove orchestration + SQS
- Delete resources: `OrchestrationStateMachine` (~386), `EventsQueue` (~125),
  `EventsDeadLetterQueue` (~115), and the state-machine IAM role that grants `EnqueueJob`
  (the `sqs:SendMessage` on `EventsQueue.Arn`, ~384).
- `ManualTriggerRole`: drop the `states:StartExecution` on `OrchestrationStateMachine`
  (~205) — keep the one on `blog-gen-content`.
- `ManualTriggerFunction`: drop `STATE_MACHINE_ARN` (~311); keep `CONTENT_STATE_MACHINE_ARN`.
  Also relax the `Require`-equivalent so only the content ARN is needed.
- Delete outputs/exports: `StateMachineArn`, `QueueUrl`, `QueueArn`, `DeadLetterQueueUrl`
  (~663–681).
- `cfn-lint infrastructure/serverless.yaml`.

### 3. `infrastructure/compute.yaml` — box = n8n + PostgreSQL + Redis only
- Remove the worker install/run from the userdata: the `blog-gen-worker` systemd unit, the
  worker-binary download from the artifacts bucket, and generation-only env
  (`QUEUE_URL` ~572, `VIDEO_STATE_MACHINE_ARN`, `OUTPUT_S3_BUCKET`, `BEDROCK_MODEL_ID`,
  `ANTHROPIC_*`, `REPOSITORIES_TABLE`, `REPO_SECRET_ID` if only the worker used them).
- Remove the instance-role statements only the worker needed: the `QueueArn` import + `sqs:*`
  (~190), `states:StartExecution` on `blog-gen-video`, `bedrock:InvokeModel`, the content-bucket
  RW + KMS, `sqs`/`ssm` for release. Keep what n8n/PG/Redis + CloudWatch/SSM-agent need.
- Remove the `QueueUrl` import (~636) and `WorkerCodeKey` param.
- **Set `INSTANCE_TYPE` → `t4g.medium`** (repo variable; manual — the box no longer runs the
  generator, so 8 GB → 4 GB is ample for n8n+PG+Redis). See [[worker-arch-arm64-graviton]].
- `cfn-lint infrastructure/compute.yaml`.

### 4. `.github/workflows/deploy.yml`
- Drop the worker build (`build worker`, ~85–86) + upload (~110) if the worker is fully removed.
- Drop `WorkerCodeKey` from the compute deploy params, and the `StateMachineArn`-related passing.
- The `CUTOVER_CONTENT` flip stays (it's now the permanent state); optionally hardcode the
  content ARN once the orchestration path is gone.

## Execution sequence (post-soak)

1. PR #A: `cmd/worker` + `compute.yaml` (+ `deploy.yml` worker build) → merge → deploy. This
   stops importing the SQS exports and removes the worker from the box.
2. Verify the box is healthy (n8n/PG/Redis up) and `/process` still works on serverless.
3. PR #B: `serverless.yaml` (remove SM/SQS/exports) → merge → deploy. Preflight/CFN deletes the
   now-unimported queue + state machine.
4. Set `INSTANCE_TYPE=t4g.medium`; redeploy compute; confirm the smaller box stays healthy.

Splitting into #A then #B enforces the cross-stack ordering and keeps each deploy verifiable.

## Rollback note

After #B, the `CUTOVER_CONTENT` rollback is gone (no orchestration SM / SQS / worker to fall
back to). Only proceed past #A once you're confident on serverless. Everything up to and
including #A is still reversible by unsetting `CUTOVER_CONTENT` and redeploying (the orchestration
SM + SQS still exist until #B).
