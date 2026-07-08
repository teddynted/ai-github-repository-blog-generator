# GitHub AI Blog Generator — build & quality targets.
#
# Single Go module (monorepo): shared code lives under internal/, and each
# Lambda entry point will live under lambdas/<fn>/ (added in later milestones).
# Lambda binaries target Linux/arm64 (provided.al2023).

GO       ?= go
LAMBDAS  := registration webhook-handler instance-starter idle-shutdown
DIST     := dist

.PHONY: all fmt vet test tidy build clean check lint-cfn

all: check

## fmt: format all Go code
fmt:
	$(GO) fmt ./...

## vet: run go vet across the module
vet:
	$(GO) vet ./...

## test: run unit tests with the race detector and coverage
test:
	$(GO) test ./... -race -cover

## tidy: sync go.mod/go.sum
tidy:
	$(GO) mod tidy

## check: the pre-commit gate (format check, vet, test)
check: vet test
	@test -z "$$(gofmt -l . )" || (echo "gofmt: files need formatting:"; gofmt -l .; exit 1)

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

## clean: remove build artifacts
clean:
	rm -rf $(DIST)
