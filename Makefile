SHELL := /bin/bash

ROOT := $(CURDIR)
CACHE := $(ROOT)/.cache
GO := $(ROOT)/.tools/go/bin/go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/desynkro/lan-party/internal/buildinfo.Version=$(VERSION)

# Everything the build touches stays inside the project.
export TMPDIR := $(ROOT)/.tmp
export GOCACHE := $(CACHE)/go-build
export GOMODCACHE := $(CACHE)/go-mod
export GOPATH := $(CACHE)/go
export XDG_CONFIG_HOME := $(ROOT)/.tmp/config
export XDG_DATA_HOME := $(ROOT)/.tmp/data
export XDG_CACHE_HOME := $(ROOT)/.tmp/cache
export PATH := $(ROOT)/.tools/go/bin:$(PATH)

.PHONY: help dirs env toolchain fmt vet test vuln build run appimage icon clean

help: ## List targets
	@awk 'BEGIN {FS = ":.*## "; print "Targets:"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dirs:
	@mkdir -p "$(CACHE)" "$(ROOT)/.tmp" "$(ROOT)/bin"

env: dirs ## Show the build environment
	@printf 'GO=%s\nGOCACHE=%s\nGOMODCACHE=%s\nGOPATH=%s\n' \
		"$(GO)" "$(GOCACHE)" "$(GOMODCACHE)" "$(GOPATH)"

toolchain: dirs ## Install the pinned Go toolchain into .tools/
	@build/toolchain.sh

fmt: toolchain ## Format Go code
	@$(GO) fmt ./...

vet: toolchain ## Vet Go code
	@$(GO) vet ./...

test: toolchain ## Run the test suite
	@$(GO) test ./...

vuln: toolchain ## Check the module for known vulnerabilities
	@$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

build: toolchain ## Build the binary into bin/
	@$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/lan-party .

run: toolchain ## Run the app with project-local config
	@$(GO) run .

appimage: toolchain ## Build the Linux AppImage into dist/
	@build/appimage.sh

icon: ## Redraw build/icon.png
	@$(GO) run ./build/icon

clean: ## Remove build output and caches
	@rm -rf "$(ROOT)/bin" "$(CACHE)" "$(ROOT)/.tmp"
