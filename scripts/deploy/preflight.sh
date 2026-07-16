#!/usr/bin/env bash
# preflight.sh — idempotent pre-deploy prep, run by deploy.yml under the deploy
# role. Automates the account-level setup and cleanup that otherwise had to be
# done by hand, so a plain `git push` reaches a clean deploy with no manual
# commands. Safe to run every deploy: every action is idempotent, and stacks are
# only deleted when stuck in a non-updatable failed state.
set -uo pipefail

PROJECT="${PROJECT:-blog-gen}"

echo "== preflight: $PROJECT =="

# The compute host is On-Demand, so no EC2 Spot service-linked role is required.

# Remove any project stack stuck in a state CloudFormation cannot update from,
# so `aws cloudformation deploy` won't abort. Healthy and rollback-complete
# (recoverable) stacks are left untouched.
STUCK="ROLLBACK_COMPLETE ROLLBACK_FAILED CREATE_FAILED UPDATE_ROLLBACK_FAILED DELETE_FAILED"
for stack in "$PROJECT-network" "$PROJECT-serverless" "$PROJECT-compute" "$PROJECT-scheduler" "$PROJECT-observability"; do
	status="$(aws cloudformation describe-stacks --stack-name "$stack" \
		--query 'Stacks[0].StackStatus' --output text 2>/dev/null || echo MISSING)"
	case " $STUCK " in
		*" $status "*)
			echo "deleting $stack (stuck in $status)"
			aws cloudformation delete-stack --stack-name "$stack"
			aws cloudformation wait stack-delete-complete --stack-name "$stack" 2>/dev/null || true
			;;
		*)
			echo "$stack: $status"
			;;
	esac
done

echo "== preflight complete =="
