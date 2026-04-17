PLUGIN=terraform-provider-graylog
VERSION=0.3.4
# Graylog version without image prefix (e.g., 5.0, 6.0, 7.0)
GRAYLOG_VERSION ?= 6.0
COMPOSE_FILE := $(CURDIR)/docker-compose.yml
COMPOSE_PROJECT_NAME ?= tf-graylog

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
    graylog-up graylog-up-graylog graylog-down graylog-stop graylog-logs graylog-wait graylog-ps test-integration test-integration-one test-integration-all \
    test-acc-integration test-acc-one test-acc-all prepare-dev-provider test-migration graylog-upgrade-seq

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
	@echo "  make graylog-stop - Stop Graylog without removing volumes (for in-place upgrades)"
	@echo "  make test-migration - Run Terraform state migration test 5→6→7 with shared state"
	@echo "  make prepare-dev-provider - Build local provider and setup Terraform dev overrides"

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
	TF_ACC=1 go test -v -tags=acceptance -run "^TestAcc" ./... -timeout 30m

# ---------- Acceptance tests with docker-compose (like integration) ----------
# Run acceptance tests against a real Graylog started by docker-compose
test-acc-integration:
	@echo "Running acceptance tests (TF_ACC=1) against docker-compose Graylog..."
	@echo "[acc] Forcing compose bring-up + wait explicitly..."
	@$(MAKE) GRAYLOG_VERSION=$(GRAYLOG_VERSION) graylog-up
	@$(MAKE) graylog-wait
	@bash -lc 'if command -v docker-compose >/dev/null 2>&1; then DC="docker-compose"; else DC="docker compose"; fi; $$DC ps || true'
	@set -e; \
	GL_BASIC=$$(printf "admin:admin" | base64); \
	export URL="$${URL:-http://127.0.0.1:9000/api}"; \
	export TOKEN="$${TOKEN:-$$GL_BASIC}"; \
	export ENABLE_OS_SNAPSHOT_ACC="$${ENABLE_OS_SNAPSHOT_ACC:-1}"; \
	export ENABLE_OS_S3_ACC="$${ENABLE_OS_S3_ACC:-1}"; \
	export ENABLE_LDAP_ACC="$${ENABLE_LDAP_ACC:-1}"; \
	TF_PATH=$$(command -v terraform || true); \
	export TF_ACC_TERRAFORM_PATH="$${TF_ACC_TERRAFORM_PATH:-$$TF_PATH}"; \
	TF_ACC=1 go test -v -tags=acceptance -run "^TestAcc" -timeout $(TIMEOUT) ./internal/provider; \
	status=$$?; \
	$(MAKE) graylog-down; \
	exit $$status

# Ensure no leftover containers/volumes before bringing the stack up
.PHONY: graylog-clean
graylog-clean:
	@echo "Cleaning up any leftover compose resources..."
	@bash -lc 'set -euo pipefail; set -x; \
	  if docker compose version >/dev/null 2>&1; then COMPOSE_BIN="docker"; COMPOSE_SUB="compose"; \
	  elif docker-compose version >/dev/null 2>&1; then COMPOSE_BIN="docker-compose"; COMPOSE_SUB=""; \
	  else echo "No docker compose found" >&2; exit 127; fi; \
	  echo "Using $$COMPOSE_BIN $$COMPOSE_SUB for compose (clean)"; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" down -v --remove-orphans || true; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" -p tf-graylog down -v --remove-orphans || true'

# Override graylog-up to depend on cleanup first
.PHONY: graylog-up
graylog-up: graylog-clean
	@echo "Starting Graylog stack via docker-compose..."
	@bash -lc 'set -euo pipefail; set -x; \
	  if docker compose version >/dev/null 2>&1; then COMPOSE_BIN="docker"; COMPOSE_SUB="compose"; \
	  elif docker-compose version >/dev/null 2>&1; then COMPOSE_BIN="docker-compose"; COMPOSE_SUB=""; \
	  else echo "No docker compose found" >&2; exit 127; fi; \
	  ver="$(GRAYLOG_VERSION)"; case "$$ver" in 5|5.) ver="5.0";; 6|6.) ver="6.0";; 7|7.) ver="7.0";; esac; \
	  mongo="$${MONGO_TAG:-7.0}"; \
	  os="$${OPENSEARCH_TAG:-2.17.1}"; \
	  echo Using MongoDB $$mongo and OpenSearch $$os for Graylog $$ver with compose file "$(COMPOSE_FILE)"; \
	  echo "Using $$COMPOSE_BIN $$COMPOSE_SUB for compose (up)"; \
	  mkdir -p ./compose/os_snapshots; \
	  chmod -R 0777 ./compose/os_snapshots || true; \
	  MONGO_TAG="$$mongo" OPENSEARCH_TAG="$$os" GRAYLOG_VERSION="$$ver" $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" up -d --remove-orphans; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" ps; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" config 1>/dev/null'

# Run acceptance tests once for the current GRAYLOG_VERSION
test-acc-one:
	@echo "GRAYLOG_VERSION=$(GRAYLOG_VERSION)"
	$(MAKE) GRAYLOG_VERSION=$(GRAYLOG_VERSION) test-acc-integration

# Run acceptance tests sequentially for Graylog 5, 6, and 7
test-acc-all:
	@set -e; \
	for ver in 5.0 6.0 7.0; do \
	  echo "==== Running acceptance tests for Graylog $$ver ===="; \
	  $(MAKE) GRAYLOG_VERSION=$$ver test-acc-one; \
	done; \
	echo "Acceptance tests passed for Graylog 5.x, 6.x and 7.x"

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

# Recreate ONLY the Graylog service (keep Mongo/OpenSearch running)
graylog-up-graylog:
	@echo "Recreating only Graylog service (keeping Mongo/OpenSearch)..."
	@bash -lc 'set -euo pipefail; set -x; \
	  if docker compose version >/dev/null 2>&1; then COMPOSE_BIN="docker"; COMPOSE_SUB="compose"; \
	  elif docker-compose version >/dev/null 2>&1; then COMPOSE_BIN="docker-compose"; COMPOSE_SUB=""; \
	  else echo "No docker compose found" >&2; exit 127; fi; \
	  ver="$(GRAYLOG_VERSION)"; case "$$ver" in 5|5.) ver="5.0";; 6|6.) ver="6.0";; 7|7.) ver="7.0";; esac; \
	  mongo="$${MONGO_TAG:-7.0}"; \
	  os="$${OPENSEARCH_TAG:-2.17.1}"; \
	  echo Using MongoDB $$mongo and OpenSearch $$os; \
	  echo "Using $$COMPOSE_BIN $$COMPOSE_SUB for compose (recreate graylog)"; \
	  MONGO_TAG="$$mongo" OPENSEARCH_TAG="$$os" GRAYLOG_VERSION="$$ver" $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" up -d --no-deps --force-recreate graylog; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" ps'

graylog-down:
	@echo "Stopping and removing Graylog stack..."
	@bash -lc 'set -euo pipefail; set -x; \
	  if docker compose version >/dev/null 2>&1; then COMPOSE_BIN="docker"; COMPOSE_SUB="compose"; \
	  elif docker-compose version >/dev/null 2>&1; then COMPOSE_BIN="docker-compose"; COMPOSE_SUB=""; \
	  else echo "No docker compose found" >&2; exit 127; fi; \
	  echo "Using $$COMPOSE_BIN $$COMPOSE_SUB for compose (down)"; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" down -v || true; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" -p tf-graylog down -v || true'

graylog-stop:
	@echo "Stopping Graylog stack (preserving volumes)..."
	@bash -lc 'set -euo pipefail; set -x; \
	  if docker compose version >/dev/null 2>&1; then COMPOSE_BIN="docker"; COMPOSE_SUB="compose"; \
	  elif docker-compose version >/dev/null 2>&1; then COMPOSE_BIN="docker-compose"; COMPOSE_SUB=""; \
	  else echo "No docker compose found" >&2; exit 127; fi; \
	  echo "Using $$COMPOSE_BIN $$COMPOSE_SUB for compose (stop)"; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" stop || true; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" -p tf-graylog stop || true'

graylog-logs:
	@echo "Graylog logs:"
	@bash -lc 'set -euo pipefail; set -x; \
	  if docker compose version >/dev/null 2>&1; then COMPOSE_BIN="docker"; COMPOSE_SUB="compose"; \
	  elif docker-compose version >/dev/null 2>&1; then COMPOSE_BIN="docker-compose"; COMPOSE_SUB=""; \
	  else echo "No docker compose found" >&2; exit 127; fi; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" logs -f graylog'

# Show docker compose services status
graylog-ps:
	@echo "Docker compose services status:"
	@bash -lc 'set -euo pipefail; set -x; \
	  if docker compose version >/dev/null 2>&1; then COMPOSE_BIN="docker"; COMPOSE_SUB="compose"; \
	  elif docker-compose version >/dev/null 2>&1; then COMPOSE_BIN="docker-compose"; COMPOSE_SUB=""; \
	  else echo "No docker compose found" >&2; exit 127; fi; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" ps'

# Wait until API is available (200 or 401 on /api/system)
graylog-wait:
	@echo "Waiting for Graylog readiness (max ~180s)..."
	@bash -lc 'set -euo pipefail; set -x; \
	  if docker compose version >/dev/null 2>&1; then COMPOSE_BIN="docker"; COMPOSE_SUB="compose"; \
	  elif docker-compose version >/dev/null 2>&1; then COMPOSE_BIN="docker-compose"; COMPOSE_SUB=""; \
	  else echo "No docker compose found" >&2; exit 127; fi; \
	  cid=$$($$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" ps -q graylog || true); \
	  for i in $$(seq 1 60); do \
	    code=$$(curl -sk -o /dev/null -w "%{http_code}" http://127.0.0.1:9000/api/system || true); \
	    health="unknown"; \
	    if [ -n "$$cid" ]; then \
	      health=$$(docker inspect -f "{{.State.Health.Status}}" $$cid 2>/dev/null || echo unknown); \
	    fi; \
	    if [ "$$code" = "200" ] || [ "$$code" = "401" ]; then \
	      echo "Graylog is ready (HTTP $$code, health=$$health). Settling 3s..."; sleep 3; exit 0; \
	    fi; \
	    echo "Waiting... attempt $$i (HTTP=$$code, health=$$health)"; sleep 3; \
	  done; \
	  echo "Graylog did not become ready in ~180s. Dumping docker status/logs..."; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" ps || true; \
	  $$COMPOSE_BIN $$COMPOSE_SUB -f "$(COMPOSE_FILE)" logs --tail=200 graylog || true; \
	  exit 1'

# Integration tests with a real Graylog
test-integration:
	@echo "Running integration tests..."
	@echo "[int] Forcing compose bring-up + wait explicitly..."
	@$(MAKE) GRAYLOG_VERSION=$(GRAYLOG_VERSION) graylog-up
	@$(MAKE) graylog-wait
	@bash -lc 'if command -v docker-compose >/dev/null 2>&1; then DC="docker-compose"; else DC="docker compose"; fi; $$DC ps || true'
	@# Basic auth admin:admin in base64
	@bash -lc '\
	  set -euo pipefail; set -x; \
	  GL_BASIC=$$(printf "admin:admin" | base64); \
	  URL="$${URL:-http://127.0.0.1:9000/api}"; \
	  TOKEN="$${TOKEN:-$$GL_BASIC}"; \
	  RUN_FLAG=""; [ -n "$(RUN)" ] && RUN_FLAG="-run $(RUN)"; \
	  export URL TOKEN; \
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

# ---------- Dev provider for Terraform CLI (dev_overrides) ----------
prepare-dev-provider:
	@echo "Building provider for dev overrides..."
	@bash -lc '\
	  set -euo pipefail; \
	  OUT_DIR="$$(pwd)/build/dev_overrides"; \
	  mkdir -p "$$OUT_DIR"; \
	  GOOS=$$(go env GOOS) GOARCH=$$(go env GOARCH) go build -o "$$OUT_DIR/terraform-provider-graylog_v0.0.0" ./; \
	  CONF_DIR="$$(pwd)/build"; \
	  CONF_FILE="$$CONF_DIR/terraformrc"; \
	  mkdir -p "$$CONF_DIR"; \
	  printf "provider_installation {\n  dev_overrides {\n    \"Ultrafenrir/graylog\" = \"%s\"\n  }\n  direct {}\n}\n" "$$OUT_DIR" > "$$CONF_FILE"; \
	  echo "Dev provider prepared at $$OUT_DIR; TF_CLI_CONFIG_FILE=$$CONF_FILE"; \
	'

# ---------- Migration test (Terraform CLI; shared local backend) ----------
test-migration:
	@echo "Preparing dev provider and Terraform CLI configuration..."
	@$(MAKE) graylog-down >/dev/null || true
	@$(MAKE) deps-fresh >/dev/null
	@$(MAKE) prepare-dev-provider >/dev/null
	@rm -f test/migration/shared/terraform.tfstate test/migration/shared/terraform.tfstate.backup || true
	@bash -lc 'set -euo pipefail; \
	  GL_BASIC=$$(printf "admin:admin" | base64); \
	  TF_VAR_url="$${URL:-http://127.0.0.1:9000/api}"; \
	  TF_VAR_token="$${TOKEN:-$$GL_BASIC}"; \
	  SUFFIX="$${SUFFIX:-$$(date +%s)}"; \
	  TF_VAR_prefix="tfm_$$SUFFIX"; \
	  TF_CLI_CONFIG_FILE="$(CURDIR)/build/terraformrc"; \
	  export TF_VAR_url TF_VAR_token TF_VAR_prefix TF_CLI_CONFIG_FILE; \
	  mkdir -p test/migration/shared; \
	  echo "==== Step 1: Graylog 5.x ===="; \
	  $(MAKE) GRAYLOG_VERSION=5.0 graylog-up; \
	  $(MAKE) graylog-wait; \
	  terraform -chdir=test/migration/step1 init -upgrade; \
	  terraform -chdir=test/migration/step1 apply -auto-approve; \
	  set +e; terraform -chdir=test/migration/step1 plan -detailed-exitcode; code=$$?; set -e; \
	  if [ "$$code" != "0" ]; then echo "Step1 plan returned $$code (expected 0)"; $(MAKE) graylog-down; exit 1; fi; \
	  echo "==== Upgrade to Graylog 6.x ===="; \
	  $(MAKE) GRAYLOG_VERSION=6.0 graylog-up-graylog; \
	  $(MAKE) graylog-wait; \
	  terraform -chdir=test/migration/step2 init -upgrade; \
	  terraform -chdir=test/migration/step2 apply -auto-approve; \
	  set +e; terraform -chdir=test/migration/step2 plan -detailed-exitcode; code=$$?; set -e; \
	  if [ "$$code" != "0" ]; then echo "Step2 plan returned $$code (expected 0)"; $(MAKE) graylog-down; exit 1; fi; \
	  echo "==== Upgrade to Graylog 7.x ===="; \
	  $(MAKE) GRAYLOG_VERSION=7.0 graylog-up-graylog; \
	  $(MAKE) graylog-wait; \
	  terraform -chdir=test/migration/step3 init -upgrade; \
	  terraform -chdir=test/migration/step3 apply -auto-approve; \
	  set +e; terraform -chdir=test/migration/step3 plan -detailed-exitcode; code=$$?; set -e; \
	  if [ "$$code" != "0" ]; then echo "Step3 plan returned $$code (expected 0)"; $(MAKE) graylog-down; exit 1; fi; \
	  if [ -z "${SKIP_DESTROY:-}" ]; then \
	    echo "==== Destroying after successful migration ===="; \
	    terraform -chdir=test/migration/step3 destroy -auto-approve || true; \
	  fi; \
	  $(MAKE) graylog-down >/dev/null; \
	  echo "Migration test passed (5→6→7)"'

# ---------- Sequential Graylog upgrade (manual diagnostics) ----------
# Starts GL 5.0 -> waits -> upgrades to 6.0 -> waits -> upgrades to 7.0 -> waits
# Uses a single MongoDB version (default MONGO_TAG=7.0) and preserves volumes between upgrades
graylog-upgrade-seq:
	@bash -lc 'set -euo pipefail; \
	  export MONGO_TAG="$${MONGO_TAG:-7.0}"; \
	  echo "[1/3] Starting Graylog 5.0 with Mongo $$MONGO_TAG"; \
	  $(MAKE) GRAYLOG_VERSION=5.0 graylog-up >/dev/null; \
	  { $(MAKE) graylog-wait >/dev/null && echo "Graylog 5.0 is up"; } || { echo "Graylog 5.0 failed to become ready"; $(MAKE) graylog-ps; $(MAKE) graylog-logs; exit 1; }; \
	  echo "[2/3] Upgrading to Graylog 6.0 (preserving volumes)"; \
	  $(MAKE) graylog-stop >/dev/null; \
	  $(MAKE) GRAYLOG_VERSION=6.0 graylog-up >/dev/null; \
	  { $(MAKE) graylog-wait >/dev/null && echo "Graylog 6.0 is up"; } || { echo "Graylog 6.0 failed to become ready"; $(MAKE) graylog-ps; $(MAKE) graylog-logs; exit 1; }; \
	  echo "[3/3] Upgrading to Graylog 7.0 (preserving volumes)"; \
	  $(MAKE) graylog-stop >/dev/null; \
	  $(MAKE) GRAYLOG_VERSION=7.0 graylog-up >/dev/null; \
	  { $(MAKE) graylog-wait >/dev/null && echo "Graylog 7.0 is up"; } || { echo "Graylog 7.0 failed to become ready"; $(MAKE) graylog-ps; $(MAKE) graylog-logs; exit 1; }; \
	  echo "Sequential upgrade succeeded (5.0 → 6.0 → 7.0)"'

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
