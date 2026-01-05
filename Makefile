PLUGIN=terraform-provider-graylog
VERSION=0.1.0
# Graylog version without image prefix (e.g., 5.0, 6.0, 7.0)
GRAYLOG_VERSION ?= 6.0

# -------- Test controls --------
# Package pattern to test (default: all)
PKG ?= ./...
# Regex to filter test names (optional)
RUN ?=
# Build tags (optional, unit tests usually run without tags)
TAGS ?=
# Use -short for unit tests by default (set to empty to disable)
SHORT ?= 1
# Go test timeout
TIMEOUT ?= 10m

OS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')

.PHONY: default deps deps-fresh fmt lint test test-unit test-acc build build-quick build-all clean release help \
	graylog-up graylog-down graylog-logs graylog-wait test-integration test-integration-one test-integration-all

default: build-quick

help:
	@echo "Available commands:"
	@echo "  make deps         - Install dependencies"
	@echo "  make deps-fresh   - Reinstall dependencies from scratch"
	@echo "  make fmt          - Format code"
	@echo "  make lint         - Lint code (go vet)"
	@echo "  make test-unit    - Run unit tests (vars: PKG, RUN, TAGS, SHORT, TIMEOUT)"
	@echo "  make test-acc     - Run acceptance tests (requires Graylog)"
	@echo "  make test-integration - Start Graylog (docker-compose) and run integration tests (vars: PKG, RUN, TIMEOUT, GRAYLOG_VERSION)"
	@echo "  make test         - Run tests: unit by default; with INTEGRATION=1 — integration"
	@echo "  make build-quick  - Fast build without dependency checks"
	@echo "  make build        - Full build with dependencies"
	@echo "  make build-all    - Build for all platforms"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make release      - Create release archive"

deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	go mod verify

deps-fresh:
	@echo "Full dependency reinstall from scratch..."
	@echo "Clearing module cache..."
	go clean -modcache
	@echo "Removing go.sum..."
	rm -f go.sum
	@echo "Downloading dependencies..."
	go mod download
	@echo "Tidying go.mod..."
	go mod tidy
	@echo "Verifying dependencies..."
	go mod verify
	@echo "Dependencies installed!"

fmt:
	@echo "Formatting code..."
	go fmt ./...

lint: fmt
	@echo "Linting code..."
	go vet ./...

# Helper for go test command with flags (no GNU Make functions; compatible with BSD make)
define GOTEST
@bash -c '\
  set -e; \
  SHORT_FLAG=""; [ -n "$(SHORT)" ] && SHORT_FLAG="-short"; \
  RUN_FLAG="";   [ -n "$(RUN)" ]   && RUN_FLAG="-run $(RUN)"; \
  TAGS_FLAG="";  [ -n "$(TAGS)" ]  && TAGS_FLAG="-tags $(TAGS)"; \
  go test -v $$SHORT_FLAG $$RUN_FLAG $$TAGS_FLAG -timeout $(TIMEOUT) $(PKG)'
endef

test-unit:
	@echo "Running unit tests..."
	$(GOTEST)

test-acc:
	@echo "Running acceptance tests (requires a running Graylog)..."
	TF_ACC=1 go test -v -run "^TestAcc" ./... -timeout 30m

test: lint
	@set -e; \
	if [ "$(INTEGRATION)" = "1" ]; then \
	  echo "Integration test mode (INTEGRATION=1)"; \
	  $(MAKE) test-integration PKG="$(PKG)" RUN="$(RUN)" TIMEOUT="$(TIMEOUT)"; \
	else \
	  $(MAKE) test-unit PKG="$(PKG)" RUN="$(RUN)" TAGS="$(TAGS)" SHORT="$(SHORT)" TIMEOUT="$(TIMEOUT)"; \
	  echo "All unit tests passed!"; \
	  echo "To run integration tests: make INTEGRATION=1 test (or make test-integration)"; \
	fi

# ---------- Graylog via docker-compose ----------
graylog-up:
	@echo "Starting Graylog stack via docker-compose..."
	@bash -c 'set -e; \
	  ver="$(GRAYLOG_VERSION)"; \
	  case "$$ver" in \
	    7.*) mongo=7.0 ;; \
	    6.*) mongo=6.0 ;; \
	    5.*) mongo=5.0 ;; \
	    *)   mongo=6.0 ;; \
	  esac; \
	  echo "Using MongoDB $$mongo for Graylog $$ver"; \
	  MONGO_TAG="$$mongo" GRAYLOG_VERSION="$$ver" docker compose up -d --remove-orphans'

graylog-down:
	@echo "Stopping and removing Graylog stack..."
	docker compose down -v

graylog-logs:
	@echo "Graylog logs:"
	docker compose logs -f graylog

# Wait until API is available (200 or 401 on /api/system)
graylog-wait:
	@echo "Waiting for Graylog API readiness..."
	@bash -c 'set -e; \
	  for i in $$(seq 1 60); do \
	    code=$$(curl -sk -o /dev/null -w "%{http_code}" http://127.0.0.1:9000/api/system || true); \
	    if [ "$$code" = "200" ] || [ "$$code" = "401" ]; then \
	      echo "Graylog is ready (HTTP $$code)"; exit 0; \
	    fi; \
	    echo "Waiting for Graylog... attempt $$i (code $$code)"; sleep 5; \
	  done; \
	  echo "Graylog did not become ready"; exit 1'

# Integration tests with a real Graylog
test-integration: graylog-up graylog-wait
	@echo "Running integration tests..."
	@# Basic auth admin:admin in base64
	@bash -c '\
	  set -e; \
	  GL_BASIC=$$(printf "admin:admin" | base64); \
	  export URL="$${URL:-http://127.0.0.1:9000/api}"; \
	  export TOKEN="$${TOKEN:-$$GL_BASIC}"; \
	  RUN_FLAG=""; [ -n "$(RUN)" ] && RUN_FLAG="-run $(RUN)"; \
	  PKG_EFF="$(PKG)"; [ -z "$$PKG_EFF" ] && PKG_EFF="./internal/..."; \
	  go test -v -tags=integration $$RUN_FLAG -timeout $(TIMEOUT) $$PKG_EFF'; \
	status=$$?; \
	$(MAKE) graylog-down; \
	exit $$status

# Run integration tests once for the current GRAYLOG_VERSION
test-integration-one:
	@echo "GRAYLOG_VERSION=$(GRAYLOG_VERSION)"
	$(MAKE) GRAYLOG_VERSION=$(GRAYLOG_VERSION) test-integration

# Run integration tests sequentially for Graylog 5, 6, and 7
test-integration-all:
	@set -e; \
	for ver in 5.0 6.0 7.0; do \
	  echo "==== Running integration tests for Graylog $$ver ===="; \
	  $(MAKE) GRAYLOG_VERSION=$$ver test-integration-one; \
	done; \
	echo "Integration tests passed for Graylog 5.x, 6.x and 7.x"

build-quick:
	@echo "Fast build $(PLUGIN) for $(OS)/$(ARCH)..."
	mkdir -p build
	go build -o build/$(PLUGIN)

build: deps lint test-unit
	@echo "Full build $(PLUGIN) for $(OS)/$(ARCH)..."
	mkdir -p build
	GOOS=$(OS) GOARCH=$(ARCH) go build -o build/$(PLUGIN)_$(VERSION)_$(OS)_$(ARCH)

build-all: deps lint test-unit
	@echo "Build for all platforms..."
	mkdir -p build
	@for os in linux darwin; do \
	  for arch in amd64 arm64; do \
	    echo "  -> $$os/$$arch"; \
	    GOOS=$$os GOARCH=$$arch go build -o build/$(PLUGIN)_$(VERSION)_$$os\_$$arch ; \
	  done; \
	done
	@echo "Build completed!"

clean:
	@echo "Cleaning..."
	rm -rf build dist

release: clean build-all
	@echo "Creating release..."
	@bash -c '\
	  set -e; \
	  mkdir -p dist; \
	  for f in build/$(PLUGIN)_$(VERSION)_*; do \
	    base=$$(basename $$f); \
	    osarch=$${base##$(PLUGIN)_$(VERSION)_}; \
	    bin_in_zip="$(PLUGIN)_v$(VERSION)"; \
	    cp "$$f" "dist/$$bin_in_zip"; \
	    (cd dist && zip "$(PLUGIN)_$(VERSION)_$$osarch.zip" "$$bin_in_zip" && rm -f "$$bin_in_zip"); \
	  done; \
	  (cd dist && sha256sum *.zip > $(PLUGIN)_$(VERSION)_SHA256SUMS); \
	  echo "Release is available in dist/ (zips + $(PLUGIN)_$(VERSION)_SHA256SUMS)"; \
	'
