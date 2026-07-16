#!/usr/bin/env bash
#
# One-time bootstrap for the GitHub AI Blog Generator deploy pipeline.
#
# Deploys infrastructure/bootstrap.yaml (artifacts bucket + GitHub OIDC provider
# + deploy role) with your admin credentials, then prints the values to set as
# GitHub Actions repository secrets/variables so the deploy workflow can run.
#
# Usage:
#   scripts/bootstrap.sh [--region us-east-1] [--project blog-gen] \
#     [--owner teddynted] [--repo ai-github-repository-blog-generator] \
#     [--no-oidc-provider] [--existing-oidc-arn ARN]
#
# An account may have only ONE OIDC provider for token.actions.githubusercontent.com.
# This script auto-detects an existing one and reuses it, so you normally don't
# need --existing-oidc-arn; the flags remain for overrides.
#
# Recovery: if a resource already exists outside the stack (e.g. the artifacts
# bucket retained from a previously deleted bootstrap stack), a plain deploy
# fails with "... already exists". Re-run with IMPORT_EXISTING=1 to adopt it
# instead of recreating it. This drives a change set with
# --import-existing-resources and first clears any leftover
# REVIEW_IN_PROGRESS/failed stack automatically:
#   IMPORT_EXISTING=1 scripts/bootstrap.sh --region ... --owner ... --repo ...
#
# Requires: AWS CLI v2 (>= 2.22 for IMPORT_EXISTING), credentials with permission
# to create the bootstrap resources (IAM role, OIDC provider, S3 bucket) and
# iam:ListOpenIDConnectProviders.

set -euo pipefail

REGION="${AWS_REGION:-us-east-1}"
PROJECT="blog-gen"
OWNER="teddynted"
REPO="ai-github-repository-blog-generator"
CREATE_OIDC="true"
EXISTING_OIDC_ARN=""
STACK_NAME=""

while [ $# -gt 0 ]; do
  case "$1" in
    --region) REGION="$2"; shift 2 ;;
    --project) PROJECT="$2"; shift 2 ;;
    --owner) OWNER="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    --no-oidc-provider) CREATE_OIDC="false"; shift ;;
    --existing-oidc-arn) EXISTING_OIDC_ARN="$2"; CREATE_OIDC="false"; shift 2 ;;
    -h|--help) sed -n '2,20p' "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 1 ;;
  esac
done

STACK_NAME="${PROJECT}-bootstrap"
HERE="$(cd "$(dirname "$0")/.." && pwd)"

# Resolve the GitHub OIDC provider. Only one per account is allowed, so if one
# already exists we must reuse it rather than fail creating a duplicate (the
# cause of "Not authorized to perform sts:AssumeRoleWithWebIdentity" when the
# role ends up trusting a non-existent/empty provider). An explicit
# --existing-oidc-arn is respected as-is.
if [ -z "$EXISTING_OIDC_ARN" ]; then
  DETECTED="$(aws iam list-open-id-connect-providers \
    --query "OpenIDConnectProviderList[?contains(Arn, 'token.actions.githubusercontent.com')].Arn | [0]" \
    --output text 2>/dev/null || true)"
  if [ -n "$DETECTED" ] && [ "$DETECTED" != "None" ]; then
    echo "Detected existing GitHub OIDC provider; reusing it:"
    echo "  $DETECTED"
    EXISTING_OIDC_ARN="$DETECTED"
    CREATE_OIDC="false"
  fi
fi

# Guard: not creating and no ARN would leave the trust policy pointing at an
# empty provider, and every role assumption would fail.
if [ "$CREATE_OIDC" = "false" ] && [ -z "$EXISTING_OIDC_ARN" ]; then
  echo "error: no GitHub OIDC provider was found or supplied." >&2
  echo "Re-run allowing creation (drop --no-oidc-provider), or pass" >&2
  echo "--existing-oidc-arn <arn>." >&2
  exit 1
fi

if [ -n "${IMPORT_EXISTING:-}" ]; then
  # Adopt resources that already exist outside the stack (e.g. an artifacts
  # bucket retained from a previously deleted bootstrap stack) instead of
  # failing with "... already exists". `aws cloudformation deploy` has no import
  # option, so drive a change set with --import-existing-resources directly
  # (AWS CLI >= 2.22).
  echo "Deploying ${STACK_NAME} in ${REGION} with resource import (create OIDC provider: ${CREATE_OIDC})..."
  STATUS="$(aws cloudformation describe-stacks --region "$REGION" --stack-name "$STACK_NAME" \
    --query 'Stacks[0].StackStatus' --output text 2>/dev/null || echo NONE)"
  # A REVIEW_IN_PROGRESS shell (a change set that never executed) or a failed
  # create holds no usable resources and cannot be updated — delete it and
  # create fresh. The retained artifacts bucket is independent and survives.
  case "$STATUS" in
    REVIEW_IN_PROGRESS|ROLLBACK_COMPLETE|CREATE_FAILED|ROLLBACK_FAILED)
      echo "  removing unusable stack ($STATUS) first (the retained bucket survives)..."
      aws cloudformation delete-stack --region "$REGION" --stack-name "$STACK_NAME"
      aws cloudformation wait stack-delete-complete --region "$REGION" --stack-name "$STACK_NAME"
      STATUS="NONE"
      ;;
  esac
  [ "$STATUS" = "NONE" ] && CS_TYPE="CREATE" || CS_TYPE="UPDATE"
  CS_NAME="bootstrap-import-$(date +%Y%m%d%H%M%S)"
  aws cloudformation create-change-set \
    --region "$REGION" \
    --stack-name "$STACK_NAME" \
    --change-set-name "$CS_NAME" \
    --change-set-type "$CS_TYPE" \
    --import-existing-resources \
    --capabilities CAPABILITY_NAMED_IAM \
    --template-body "file://${HERE}/infrastructure/bootstrap.yaml" \
    --parameters \
      ParameterKey=ProjectName,ParameterValue="$PROJECT" \
      ParameterKey=GitHubOwner,ParameterValue="$OWNER" \
      ParameterKey=GitHubRepo,ParameterValue="$REPO" \
      ParameterKey=CreateOIDCProvider,ParameterValue="$CREATE_OIDC" \
      ParameterKey=ExistingOIDCProviderArn,ParameterValue="$EXISTING_OIDC_ARN"
  aws cloudformation wait change-set-create-complete \
    --region "$REGION" --stack-name "$STACK_NAME" --change-set-name "$CS_NAME"
  aws cloudformation execute-change-set \
    --region "$REGION" --stack-name "$STACK_NAME" --change-set-name "$CS_NAME"
  aws cloudformation wait "stack-$(printf '%s' "$CS_TYPE" | tr '[:upper:]' '[:lower:]')-complete" \
    --region "$REGION" --stack-name "$STACK_NAME"
else
  echo "Deploying ${STACK_NAME} in ${REGION} (create OIDC provider: ${CREATE_OIDC})..."
  aws cloudformation deploy \
    --region "$REGION" \
    --template-file "${HERE}/infrastructure/bootstrap.yaml" \
    --stack-name "$STACK_NAME" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides \
      ProjectName="$PROJECT" \
      GitHubOwner="$OWNER" \
      GitHubRepo="$REPO" \
      CreateOIDCProvider="$CREATE_OIDC" \
      ExistingOIDCProviderArn="$EXISTING_OIDC_ARN"
fi

echo
echo "Bootstrap complete. Set these on the GitHub repository"
echo "(Settings -> Secrets and variables -> Actions):"
echo

get() {
  aws cloudformation describe-stacks --region "$REGION" --stack-name "$STACK_NAME" \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text
}

echo "  secret   AWS_DEPLOY_ROLE_ARN = $(get DeployRoleArn)"
echo "  variable ARTIFACTS_BUCKET    = $(get ArtifactsBucketName)"
echo "  variable AWS_REGION          = ${REGION}"
echo "  variable KEY_PAIR_NAME       = <your EC2 key pair>"
echo "  variable OPERATOR_CIDR       = <your SSH source CIDR>  (optional)"
echo "  variable DEPLOY_ENABLED      = true"
echo
echo "Then merge to main (or run the deploy workflow) to deploy the app stacks."
