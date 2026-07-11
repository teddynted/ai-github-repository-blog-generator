#!/usr/bin/env bash
#
# Build, package, and deploy the scheduled EC2 start/stop stack
# (infrastructure/scheduler.yaml) — two Go Lambdas + two EventBridge Schedules
# that power a specific instance on at 18:00 and off at 20:00 (configurable).
#
# Usage:
#   INSTANCE_ID=i-0123456789abcdef0 scripts/deploy-scheduler.sh
#
# Environment / flags (env vars; all but INSTANCE_ID have defaults):
#   INSTANCE_ID   (required) EC2 instance to schedule.
#   REGION        AWS region                    (default: aws configure / us-east-1)
#   PROJECT_NAME  resource prefix + tag         (default: blog-gen)
#   STACK_NAME    CloudFormation stack name     (default: <project>-scheduler)
#   TIMEZONE      IANA timezone for the cron    (default: Etc/UTC)
#   START_EXPR    start cron                    (default: cron(0 18 * * ? *))
#   STOP_EXPR     stop cron                     (default: cron(0 20 * * ? *))
#   ENVIRONMENT   environment tag (dev|staging|prod, default: dev)
#   BUCKET        artifacts S3 bucket           (default: blog-gen-artifacts-<account>-<region>)
set -euo pipefail

: "${INSTANCE_ID:?set INSTANCE_ID=i-0123... (the instance to schedule)}"

REGION="${REGION:-$(aws configure get region 2>/dev/null || echo us-east-1)}"
PROJECT_NAME="${PROJECT_NAME:-blog-gen}"
STACK_NAME="${STACK_NAME:-${PROJECT_NAME}-scheduler}"
TIMEZONE="${TIMEZONE:-Etc/UTC}"
START_EXPR="${START_EXPR:-cron(0 18 * * ? *)}"
STOP_EXPR="${STOP_EXPR:-cron(0 20 * * ? *)}"
ENVIRONMENT="${ENVIRONMENT:-dev}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"

ACCOUNT="$(aws sts get-caller-identity --query Account --output text)"
BUCKET="${BUCKET:-${PROJECT_NAME}-artifacts-${ACCOUNT}-${REGION}}"

# Version the S3 keys by content hash so a redeploy always ships fresh code
# (CloudFormation only updates a function when its S3Key changes).
build_zip() {
  local fn="$1"
  echo "building $fn (linux/arm64)..." >&2
  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build \
    -C "$ROOT" -o "$DIST/$fn/bootstrap" "./lambdas/$fn"
  (cd "$DIST/$fn" && zip -q -j "../$fn.zip" bootstrap)
  # Print the sha-prefixed key on stdout for the caller.
  local sha
  sha="$(shasum -a 256 "$DIST/$fn.zip" | cut -c1-12)"
  local key="scheduler/${sha}-${fn}.zip"
  aws s3 cp "$DIST/$fn.zip" "s3://$BUCKET/$key" --region "$REGION" >&2
  echo "$key"
}

START_KEY="$(build_zip scheduled-start)"
STOP_KEY="$(build_zip scheduled-stop)"

echo "deploying stack $STACK_NAME (instance $INSTANCE_ID, tz $TIMEZONE)..." >&2
aws cloudformation deploy \
  --region "$REGION" \
  --template-file "$ROOT/infrastructure/scheduler.yaml" \
  --stack-name "$STACK_NAME" \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides \
    ProjectName="$PROJECT_NAME" \
    Environment="$ENVIRONMENT" \
    InstanceId="$INSTANCE_ID" \
    ScheduleTimezone="$TIMEZONE" \
    StartExpression="$START_EXPR" \
    StopExpression="$STOP_EXPR" \
    ArtifactsBucket="$BUCKET" \
    StartCodeKey="$START_KEY" \
    StopCodeKey="$STOP_KEY"

echo "done. Schedule window:" >&2
aws cloudformation describe-stacks --region "$REGION" --stack-name "$STACK_NAME" \
  --query "Stacks[0].Outputs[?OutputKey=='ScheduleWindow'].OutputValue" --output text
