# tasks — self-hosted, Dolt-free task tracker (beads replacement).
GO      ?= go
BINDIR  ?= bin
PORT    ?= 7842
DB      ?= data/tasks.db
# Default import source: the forge repo's beads export.
IMPORT  ?= ../forge-crafting-intepreters/.beads/issues.jsonl
EXPORT  ?= ../forge-crafting-intepreters/.beads/issues.jsonl

# Build metadata (stamped into the binary via -ldflags).
# --verify fails silently on a repo with no commits (plain `rev-parse HEAD`
# would print "HEAD" and inject a newline into the ldflags).
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --verify --short=12 HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PKG      = github.com/tristanMatthias/tasks/internal/buildinfo
LDFLAGS  = -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)
IMAGE   ?= ghcr.io/tristanmatthias/tasks

.PHONY: all build tasksd tasks test cover cover-html cover-check hooks vet fmt lint import serve funnel serve-tailnet install docker docker-run clean

all: build

build: tasksd tasks

tasksd:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINDIR)/tasksd ./cmd/tasksd

tasks:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINDIR)/tasks ./cmd/tasks
	@ln -sf tasks $(BINDIR)/bd   # bd -> tasks (backward-compatible symlink)

test:
	$(GO) test ./...

# Accurate integrated coverage total (unions duplicate blocks; never fails).
cover:
	@COVERAGE_MIN=0 ./scripts/check-coverage.sh

cover-html:
	@COVERAGE_MIN=0 ./scripts/check-coverage.sh >/dev/null
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "wrote coverage.html"

# Coverage gate (used by the pre-commit hook). Fails below the threshold in
# .coverage-min (or COVERAGE_MIN).
cover-check:
	./scripts/check-coverage.sh

# Install the git hooks (coverage gate on pre-commit).
hooks:
	git config core.hooksPath .githooks
	@echo "git hooks enabled (.githooks) — commits now run the coverage gate"

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

# Static-analysis (uses staticcheck if installed).
lint:
	$(GO) vet ./...
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "(staticcheck not installed; skipping)"

# Build the container image (multi-arch static binary inside).
docker:
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg DATE=$(DATE) -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

docker-run: docker
	docker run --rm -p 127.0.0.1:$(PORT):$(PORT) -e TASKS_TOKEN=$${TASKS_TOKEN:?set TASKS_TOKEN} -v tasks-data:/data $(IMAGE):latest

# One-shot import of an existing beads issues.jsonl into the SQLite store.
import: tasksd
	$(BINDIR)/tasksd import --db $(DB) $(IMPORT)

# Run the server locally (needs TASKS_TOKEN in the environment to enable auth).
serve: build
	$(BINDIR)/tasksd --addr 127.0.0.1:$(PORT) --db $(DB) --export $(EXPORT)

# Expose the local server to the public internet over HTTPS via Tailscale Funnel
# (required so Claude Code web can reach the MCP endpoint). Run `make serve` too.
funnel:
	tailscale funnel $(PORT)

# Tailnet-only (no public internet) — reachable only from your own devices.
serve-tailnet:
	tailscale serve --bg $(PORT)

# Install binaries (and the bd symlink) into $(PREFIX)/bin.
PREFIX ?= $(HOME)/.local
install: build
	install -d $(PREFIX)/bin
	install -m 0755 $(BINDIR)/tasksd $(PREFIX)/bin/tasksd
	install -m 0755 $(BINDIR)/tasks  $(PREFIX)/bin/tasks
	ln -sf tasks $(PREFIX)/bin/bd
	@echo "installed tasksd, tasks, and bd -> $(PREFIX)/bin"

clean:
	rm -rf $(BINDIR) coverage.out coverage.html
