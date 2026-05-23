# wachat — local-only gates. No CI by design; everything runs here.
#
# `make check` is the gate. It is what the pre-commit hook runs.

GO        ?= go
GOFMT     ?= gofmt
BINARY    ?= wachat
LDFLAGS   ?= -s -w

# Use an in-repo temp dir for go-build artifacts. On some Windows machines
# antivirus blocks newly-created executables under %TEMP%; pointing GOTMPDIR
# at a workspace directory avoids the issue with zero downside elsewhere.
export GOTMPDIR := $(CURDIR)/.tmp-go

# OS-specific binary suffix (Windows .exe)
ifeq ($(OS),Windows_NT)
EXE := .exe
else
EXE :=
endif

.PHONY: all help tidy fmt fmt-check vet test test-short test-race cover check hooks build run clean

all: check ## default: run the gate

help: ## show available targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

tidy: ## go mod tidy
	$(GO) mod tidy

fmt: ## format the tree
	$(GOFMT) -w .

fmt-check: ## fail if anything is unformatted
	@out=$$($(GOFMT) -l .); \
	if [ -n "$$out" ]; then \
		echo "gofmt: the following files need formatting:"; \
		echo "$$out"; \
		exit 1; \
	fi

vet: ## go vet
	$(GO) vet ./...

test: ## run tests (works everywhere, no cgo required)
	$(GO) test ./... -count=1

test-short: ## quick tests, skip long benchmarks
	$(GO) test ./... -short -count=1

test-race: ## race detector — requires a C toolchain (cgo). Opt-in.
	CGO_ENABLED=1 $(GO) test ./... -race -count=1

cover: ## coverage report
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out | tail -1

check: fmt-check vet test ## the gate — fmt, vet, tests

hooks: ## install the local pre-commit hook
	@bash scripts/install-hooks.sh

build: ## build a stripped release binary for the host platform
	$(GO) build -ldflags="$(LDFLAGS) -X main.Version=$(VERSION)" -o $(BINARY)$(EXE) .

# VERSION is embedded into the binary via -ldflags. Defaults to the
# most recent annotated tag; override at the command line for snapshot
# builds (`make build VERSION=dev`).
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

dist: ## cross-compile release artifacts into dist/ for the current host's reachable targets
	@mkdir -p dist
	@echo "building $(VERSION) for windows/amd64..."
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS) -X main.Version=$(VERSION)" -o dist/$(BINARY)-$(VERSION)-windows-amd64.exe .
	@echo "building $(VERSION) for windows/arm64..."
	@CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS) -X main.Version=$(VERSION)" -o dist/$(BINARY)-$(VERSION)-windows-arm64.exe .
	@echo "(linux/macos builds need a C toolchain — Gio uses cgo on those platforms;"
	@echo " run \"make dist\" on a Linux/macOS host to add them, or enable the optional"
	@echo " release workflow at .github/workflows/release.yml.disabled)"
	@cd dist && sha256sum $(BINARY)-$(VERSION)-* > SHA256SUMS-$(VERSION).txt
	@ls -la dist/

publish: dist ## upload dist/ artifacts to the matching GitHub release
	@gh release upload $(VERSION) dist/$(BINARY)-$(VERSION)-* dist/SHA256SUMS-$(VERSION).txt --clobber

.PHONY: dist publish

run: ## go run the app
	$(GO) run .

bench: ## seed a 100k-msg DB and print perf numbers
	$(GO) run ./cmd/bench

clean: ## remove build artifacts and local state
	rm -f $(BINARY) $(BINARY).exe coverage.out coverage.html
	rm -f wachat.db wachat.db-journal wachat.db-wal wachat.db-shm
	rm -rf media/ .tmp-go/

$(shell mkdir -p .tmp-go)
