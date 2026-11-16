SHELL := /bin/bash

ROOT := $(CURDIR)
CACHE := $(ROOT)/.cache
GO := $(ROOT)/.tools/go/bin/go
WAILS := $(CACHE)/go/bin/wails3
WAILS_VERSION := v3.1.0

export TMPDIR := $(ROOT)/.tmp
export GOCACHE := $(CACHE)/go-build
export GOMODCACHE := $(CACHE)/go-mod
export GOPATH := $(CACHE)/go
export PATH := $(ROOT)/.tools/go/bin:$(PATH)

.PHONY: help dirs env toolchain wails fmt vet test build dev package clean

help: ## List targets
	@awk 'BEGIN {FS = ":.*## "; print "Targets:"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dirs:
	@mkdir -p "$(CACHE)" "$(ROOT)/.tmp" "$(ROOT)/bin"

env: dirs ## Show the build environment
	@printf 'GO=%s\nGOCACHE=%s\nGOMODCACHE=%s\nGOPATH=%s\n' \
		"$(GO)" "$(GOCACHE)" "$(GOMODCACHE)" "$(GOPATH)"

toolchain: dirs ## Install the pinned Go toolchain into .tools/
	@scripts/toolchain.sh

wails: toolchain ## Install the pinned Wails CLI into .cache/
	@test -x "$(WAILS)" || $(GO) install github.com/wailsapp/wails/v3/cmd/wails3@$(WAILS_VERSION)

fmt: toolchain ## Format Go code
	@$(GO) fmt ./...

vet: toolchain ## Vet Go code
	@$(GO) vet ./...

test: toolchain ## Run the test suite
	@$(GO) test ./...

build: wails ## Build the app for this platform
	@$(WAILS) build

dev: wails ## Run with hot reload
	@$(WAILS) dev

package: wails ## Build release packages (AppImage, exe)
	@$(WAILS) package

clean: ## Remove build output and caches
	@rm -rf "$(ROOT)/bin" "$(CACHE)" "$(ROOT)/.tmp"
