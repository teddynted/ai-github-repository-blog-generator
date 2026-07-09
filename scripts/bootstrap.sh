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
# Requires: AWS CLI v2, credentials with permission to create the bootstrap
# resources (IAM role, OIDC provider, S3 bucket) and iam:ListOpenIDConnectProviders.

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
