# diskdiet — developer task runner. Run `make help` to list targets.

SHELL         := /bin/bash
.SHELLFLAGS   := -eu -o pipefail -c
.DEFAULT_GOAL := help

MODULE   := github.com/roystondwork-dev/diskdiet
BIN      := bin/diskdiet
TOOL     := go tool -modfile=tools/go.mod
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -s -w \
	-X $(MODULE)/internal/buildinfo.version=$(VERSION) \
	-X $(MODULE)/internal/buildinfo.commit=$(COMMIT) \
	-X $(MODULE)/internal/buildinfo.date=$(DATE)

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build diskdiet into ./bin.
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/diskdiet

.PHONY: run
run: ## Run diskdiet with ARGS="...".
	go run ./cmd/diskdiet $(ARGS)

.PHONY: test
test: ## Run tests with the race detector.
	go test -race -shuffle=on -count=1 ./...

.PHONY: cover
cover: ## Run tests and enforce the coverage gate.
	go test -count=1 -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
	$(TOOL) go-test-coverage --config=.testcoverage.yml

.PHONY: lint
lint: ## Lint Go code and GitHub Actions workflows.
	golangci-lint run ./...
	$(TOOL) actionlint

.PHONY: vuln
vuln: ## Check for known vulnerabilities.
	$(TOOL) govulncheck ./...

.PHONY: tidy
tidy: ## Tidy go.mod files.
	go mod tidy
	cd tools && go mod tidy

.PHONY: snapshot
snapshot: ## Build release artifacts locally without publishing.
	goreleaser release --snapshot --clean --skip=sign,sbom

FUZZTIME ?= 30s

.PHONY: fuzz
fuzz: ## Run every fuzz target for FUZZTIME each.
	scripts/fuzz.sh $(FUZZTIME)

.PHONY: mutation
mutation: ## Mutation-test the safety-critical packages (slow); fails below 75% efficacy.
	scripts/mutation.sh ./internal/fsx 75
	scripts/mutation.sh ./internal/source 75
	scripts/mutation.sh ./internal/units 75

.PHONY: check
check: lint test vuln ## Everything CI checks.

##@ Desktop app (needs Node.js and the wails3 CLI: go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0)

.PHONY: app
app: ## Build the desktop app into app/bin.
	cd app && wails3 build

.PHONY: app-dev
app-dev: ## Run the desktop app with live reload.
	cd app && wails3 dev

.PHONY: app-package
app-package: ## Package the desktop app for this OS (dmg, installer or AppImage).
	cd app && wails3 task package

.PHONY: demo
demo: build ## Record docs/assets/demo.gif with made-up data (needs vhs).
	vhs docs/assets/demo.tape

##@ Documentation site (MkDocs Material; pip install --require-hashes -r docs/requirements.txt)

.PHONY: docs-cli
docs-cli: ## Regenerate the CLI reference in docs/commands.
	go run ./internal/cli/gendocs -out docs/commands

.PHONY: docs
docs: docs-cli ## Build the site into ./site (strict).
	mkdocs build --strict

.PHONY: docs-serve
docs-serve: docs-cli ## Preview the site at http://127.0.0.1:8000.
	mkdocs serve
