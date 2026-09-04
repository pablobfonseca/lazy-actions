BINARY := lazyactions
BIN_DIR := bin
INSTALL_DIR := $(HOME)/.local/bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build run test vet fmt check install clean release help

## build: compile binary to ./bin/lazyactions
build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

## run: run against ./config.yaml (or ~/.config/gh-action-monitor/config.yaml)
run:
	go run .

## test: run all package tests
test:
	go test ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: run gofmt on all Go files
fmt:
	gofmt -w -s .

## check: fmt + vet + test (what CI should run)
check: fmt vet test

## install: build and copy binary to ~/.local/bin/lazyactions
install: build
	@mkdir -p $(INSTALL_DIR)
	cp $(BIN_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR)

## release: publish the pushed tag with goreleaser (needs gh auth)
release:
	GITHUB_TOKEN="$$(gh auth token)" HOMEBREW_TAP_TOKEN="$$(gh auth token)" goreleaser release --clean

## help: list targets
help:
	@awk '/^## / { sub(/^## /, ""); printf "  %s\n", $$0 }' $(MAKEFILE_LIST)
