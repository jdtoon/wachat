# wachat — local-only gates. No CI by design; everything runs here.
#
# `make check` is the gate. It is what the pre-commit hook runs.

GO        ?= go
GOFMT     ?= gofmt
BINARY    ?= wachat
LDFLAGS   ?= -s -w

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

build: ## build a stripped release binary
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY)$(EXE) .

run: ## go run the app
	$(GO) run .

clean: ## remove build artifacts and local state
	rm -f $(BINARY) $(BINARY).exe coverage.out coverage.html
	rm -f wachat.db wachat.db-journal wachat.db-wal wachat.db-shm
	rm -rf media/
