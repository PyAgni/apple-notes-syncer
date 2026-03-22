BINARY_NAME := apple-notes-sync
BINARY_DIR := bin
COVERAGE_FILE := coverage.out
COVERAGE_THRESHOLD := 80
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags "-X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev) \
                      -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) \
                      -X main.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"

PLIST_NAME := com.apple-notes-sync.plist
PLIST_SRC := launchd/$(PLIST_NAME)
PLIST_DEST := $(HOME)/Library/LaunchAgents/$(PLIST_NAME)

.PHONY: all build clean test test-coverage check-coverage cover lint fmt vet tidy run install launchd unlaunchd help

all: lint test build ## Run lint, test, and build

build: ## Build the binary
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/apple-notes-sync/

install: build ## Install to /usr/local/bin
	cp $(BINARY_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to /usr/local/bin/"

run: build ## Build and run
	./$(BINARY_DIR)/$(BINARY_NAME)

clean: ## Remove build artifacts
	rm -rf $(BINARY_DIR) $(COVERAGE_FILE)

test: ## Run tests with race detector
	$(GO) test ./... -race -count=1

test-coverage: ## Run tests with coverage
	$(GO) test ./... -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic -count=1

check-coverage: test-coverage ## Check that coverage meets threshold
	@coverage=$$($(GO) tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}' | tr -d '%'); \
	echo "Total coverage: $${coverage}%"; \
	threshold=$(COVERAGE_THRESHOLD); \
	if [ $$(echo "$${coverage} < $${threshold}" | bc -l) -eq 1 ]; then \
		echo "FAIL: Coverage $${coverage}% is below threshold $${threshold}%"; \
		exit 1; \
	else \
		echo "OK: Coverage $${coverage}% meets threshold $${threshold}%"; \
	fi

cover: test-coverage ## Open coverage report in browser
	$(GO) tool cover -html=$(COVERAGE_FILE)

lint: ## Run linters (go vet + staticcheck)
	$(GO) vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping (install: go install honnef.co/go/tools/cmd/staticcheck@latest)"

fmt: ## Format code
	$(GO) fmt ./...
	@which goimports > /dev/null 2>&1 && goimports -w . || echo "goimports not installed, skipping"

vet: ## Run go vet
	$(GO) vet ./...

tidy: ## Tidy and verify go modules
	$(GO) mod tidy
	$(GO) mod verify

launchd: ## Install and load the launchd plist
	@if [ ! -f $(PLIST_SRC) ]; then echo "Error: $(PLIST_SRC) not found"; exit 1; fi
	@mkdir -p $(HOME)/Library/LaunchAgents
	cp $(PLIST_SRC) $(PLIST_DEST)
	launchctl load $(PLIST_DEST)
	@echo "Loaded $(PLIST_NAME)"

unlaunchd: ## Unload and remove the launchd plist
	launchctl unload $(PLIST_DEST) 2>/dev/null || true
	rm -f $(PLIST_DEST)
	@echo "Unloaded and removed $(PLIST_NAME)"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
