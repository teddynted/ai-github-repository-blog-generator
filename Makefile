# GitHub AI Blog Generator — build & quality targets.
#
# Single Go module (monorepo): shared code lives under internal/, and each
# Lambda entry point will live under lambdas/<fn>/ (added in later milestones).
# Lambda binaries target Linux/arm64 (provided.al2023).

GO       ?= go
LAMBDAS  := registration webhook-handler instance-starter idle-shutdown scheduled-start scheduled-stop
DIST     := dist

HOOKS    := scripts/hooks

.PHONY: all fmt fmt-check vet lint test tidy build build-worker build-release release clean check lint-cfn hooks act deploy-scheduler

all: check

## hooks: install the local Git hooks (core.hooksPath = .githooks)
hooks:
	./scripts/install-hooks.sh

## fmt: format all Go code (gofmt, and goimports if installed)
fmt:
	$(HOOKS)/format.sh

## fmt-check: fail if any Go file is not gofmt-clean (CI parity)
fmt-check:
	$(HOOKS)/format.sh --check

## vet: run go vet across the module
vet:
	$(GO) vet ./...

## lint: static analysis (go vet, and golangci-lint if installed)
lint:
	$(HOOKS)/lint.sh

## test: run unit tests with the race detector and coverage
test:
	GO_TEST_FLAGS="-race -cover" $(HOOKS)/tests.sh

## tidy: sync go.mod/go.sum
tidy:
	$(GO) mod tidy

## act: run the primary CI workflow locally with act (needs Docker + act)
act:
	$(HOOKS)/run-act.sh

## check: the pre-commit gate (format check, lint, test)
check: fmt-check lint test

## lint-cfn: validate the CloudFormation templates
lint-cfn:
	cfn-lint infrastructure/*.yaml

## build: compile every Lambda to dist/<fn>/bootstrap (Linux/arm64)
build:
	@for fn in $(LAMBDAS); do \
		if [ -d "lambdas/$$fn" ]; then \
			echo "building $$fn"; \
			GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build \
				-o $(DIST)/$$fn/bootstrap ./lambdas/$$fn || exit 1; \
		else \
			echo "skip $$fn (not implemented yet)"; \
		fi; \
	done

## build-worker: compile the instance worker and approvals CLI (Linux/amd64)
build-worker:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(DIST)/worker/worker ./cmd/worker
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(DIST)/worker/approve ./cmd/approve

## build-release: compile the release management CLI for the host platform
build-release:
	$(GO) build -o $(DIST)/release ./cmd/release

## release: run the release CLI (e.g. make release ARGS="--dry-run minor")
release: build-release
	$(DIST)/release $(ARGS)

## deploy-scheduler: build, package, and deploy the scheduled start/stop stack
## Usage: make deploy-scheduler INSTANCE_ID=i-0123... [REGION=us-east-1] [TIMEZONE=Africa/Johannesburg]
deploy-scheduler:
	./scripts/deploy-scheduler.sh

## clean: remove build artifacts
clean:
	rm -rf $(DIST)
