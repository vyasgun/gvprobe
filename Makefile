MODULE := github.com/vyasgun/gvprobe
VERSION_PKG := $(MODULE)/internal/version

# Override on CLI: make build VERSION=1.2.3
VERSION ?= $(shell TAG=$$(git describe --tags --always --dirty 2>/dev/null); \
	if [ -n "$$TAG" ]; then echo "$$TAG" | sed 's/^v//'; else echo "0.0.0-dev"; fi)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null; true)

LDFLAGS := \
	-X '$(VERSION_PKG).Version=$(VERSION)' \
	-X '$(VERSION_PKG).Commit=$(COMMIT)'

.PHONY: build
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/gvprobe .

.PHONY: install
install:
	go install -trimpath -ldflags "$(LDFLAGS)" .

lint:
	go get github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...

fmt:
	go fmt ./...