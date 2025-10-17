PLUGIN=terraform-provider-graylog
VERSION=0.1.0

OS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')

default: build

deps:
	go mod tidy
	go mod verify

fmt:
	go fmt ./...

lint:
	go vet ./...

test:
	@echo "Running tests..."
	go test ./... -v

build: deps
	@echo "Building $(PLUGIN) for $(OS)/$(ARCH)..."
	mkdir -p build
	GOOS=$(OS) GOARCH=$(ARCH) go build -o build/$(PLUGIN)_$(VERSION)_$(OS)_$(ARCH)

build-all: deps
	@echo "Building for all platforms..."
	mkdir -p build
	for os in linux darwin; do \
	  for arch in amd64 arm64; do \
	    echo "  -> $$os/$$arch"; \
	    GOOS=$$os GOARCH=$$arch go build -o build/$(PLUGIN)_$(VERSION)_$$os\_$$arch ; \
	  done; \
	done

clean:
	rm -rf build dist

release: clean build-all
	mkdir -p dist
	zip -r dist/$(PLUGIN)_$(VERSION).zip . -x "*.git*" "dist/*" "build/*"
	@echo "dist/$(PLUGIN)_$(VERSION).zip ready"
