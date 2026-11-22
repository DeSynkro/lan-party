SHELL := /bin/bash

ROOT := $(CURDIR)
CACHE := $(ROOT)/.cache
GO := $(ROOT)/.tools/go/bin/go
VERSION ?= $(shell git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null | sed -e "s/^v//" -e "s/^$$/dev/")
LDFLAGS := -s -w -X github.com/desynkro/lan-party/internal/buildinfo.Version=$(VERSION)
DOCKER ?= docker
RELAY_IMAGE ?= ghcr.io/desynkro/lan-party-relay
RELAY_TAG ?= $(VERSION)
WEB_IMAGE ?= ghcr.io/desynkro/lan-party-web
WEB_TAG ?= $(VERSION)

# Everything the build touches stays inside the project.
export TMPDIR := $(ROOT)/.tmp
export GOCACHE := $(CACHE)/go-build
export GOMODCACHE := $(CACHE)/go-mod
export GOPATH := $(CACHE)/go
export GOBIN := $(ROOT)/.tools/bin
export XDG_CONFIG_HOME := $(ROOT)/.tmp/config
export XDG_DATA_HOME := $(ROOT)/.tmp/data
export XDG_CACHE_HOME := $(ROOT)/.tmp/cache
export PATH := $(ROOT)/.tools/go/bin:$(PATH)

.PHONY: help dirs env toolchain tidy fmt vet test test-race vuln build run gtk-probe appimage relay relay-native relay-bin relay-image web web-test web-native web-bin web-image clean

help: ## List targets
	@awk 'BEGIN {FS = ":.*## "; print "Targets:"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dirs:
	@mkdir -p "$(CACHE)" "$(ROOT)/.tmp" "$(ROOT)/bin"

env: dirs ## Show the build environment
	@printf 'GO=%s\nGOCACHE=%s\nGOMODCACHE=%s\nGOPATH=%s\nGOBIN=%s\n' \
		"$(GO)" "$(GOCACHE)" "$(GOMODCACHE)" "$(GOPATH)" "$(GOBIN)"

toolchain: dirs ## Install the pinned Go toolchain into .tools/
	@build/toolchain.sh

tidy: toolchain ## Update go.mod and go.sum
	@$(GO) mod tidy

fmt: toolchain ## Format Go code
	@$(GO) fmt ./...

vet: toolchain ## Vet Go code
	@$(GO) vet ./...

test: toolchain ## Run the test suite
	@$(GO) test ./...

test-race: toolchain ## Run the test suite with the race detector
	@$(GO) test -race ./...

vuln: toolchain ## Check the module for known vulnerabilities
	@$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

build: toolchain ## Build the binary into bin/
	@$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/lan-party ./cmd/lan-party

run: toolchain ## Run the app with project-local config
	@$(GO) run ./cmd/lan-party

gtk-probe: toolchain ## Run the GTK4 interface probe
	@$(GO) run ./cmd/gtk-probe

appimage: toolchain ## Build the Linux AppImage into dist/ (system GTK4)
	@VERSION=$(VERSION) build/appimage.sh

relay: toolchain ## Build the relay binary and host-arch release zip
	@VERSION=$(VERSION) ARCHES=host build/relay.sh

relay-native: toolchain ## Build Linux amd64/arm64 relay release zips
	@VERSION=$(VERSION) ARCHES="amd64 arm64" build/relay.sh

relay-bin: toolchain ## Build the host-arch relay binary into bin/
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/lan-party-relay ./cmd/lan-party-relay

relay-image: relay-bin ## Build the relay container image from the host binary
	@tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
		cp bin/lan-party-relay "$$tmp/lan-party-relay"; \
		$(DOCKER) build --build-arg VERSION=$(VERSION) -f relay/docker/Dockerfile \
			-t $(RELAY_IMAGE):$(RELAY_TAG) "$$tmp"

web: toolchain ## Run the web host tests and build it into bin/
	@$(GO) test ./server/...
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/lan-party-web ./server/cmd/lan-party-web

web-test: toolchain ## Run the web host tests
	@$(GO) test ./server/...

web-native: toolchain ## Build Linux amd64/arm64 web host release zips
	@VERSION=$(VERSION) ARCHES="amd64 arm64" build/web.sh

web-bin: toolchain ## Build the host-arch web host binary into bin/
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/lan-party-web ./server/cmd/lan-party-web

web-image: ## Build the web host container image from the repository root
	@$(DOCKER) build --build-arg VERSION=$(VERSION) -f server/docker/Dockerfile \
		-t $(WEB_IMAGE):$(WEB_TAG) .

clean: ## Remove build output and caches
	@rm -rf "$(ROOT)/bin" "$(CACHE)" "$(ROOT)/.tmp"
