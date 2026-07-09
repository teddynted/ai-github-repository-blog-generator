#!/usr/bin/env bash
# preflight.sh — idempotent pre-deploy prep, run by deploy.yml under the deploy
# role. Automates the account-level setup and cleanup that otherwise had to be
# done by hand, so a plain `git push` reaches a clean deploy with no manual
# commands. Safe to run every deploy: every action is idempotent, and stacks are
# only deleted when stuck in a non-updatable failed state.
set -uo pipefail

PROJECT="${PROJECT:-blog-gen}"

echo "== preflight: $PROJECT =="

# 1. EC2 Spot service-linked role — required the first time any Spot instance
#    launches in the account. Best-effort: if the role exists or the deploy role
#    lacks permission (already created out-of-band), carry on.
if aws iam get-role --role-name AWSServiceRoleForEC2Spot >/dev/null 2>&1; then
	echo "spot service-linked role: present"
else
	if aws iam create-service-linked-role --aws-service-name spot.amazonaws.com >/dev/null 2>&1; then
		echo "spot service-linked role: created"
	else
		echo "spot service-linked role: could not create (may already exist); continuing"
	fi
fi

# 2. Remove any project stack stuck in a state CloudFormation cannot update from,
#    so `aws cloudformation deploy` won't abort. Healthy and rollback-complete
#    (recoverable) stacks are left untouched.
STUCK="ROLLBACK_COMPLETE ROLLBACK_FAILED CREATE_FAILED UPDATE_ROLLBACK_FAILED DELETE_FAILED"
for stack in "$PROJECT-network" "$PROJECT-serverless" "$PROJECT-compute" "$PROJECT-observability"; do
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
