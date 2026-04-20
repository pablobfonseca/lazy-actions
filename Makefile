BINARY := lazy-actions
BIN_DIR := bin

.PHONY: build run test vet fmt check install clean help

## build: compile binary to ./bin/lazy-actions
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) .

## run: run against ./config.yaml (or ~/.config/lazy-actions/config.yaml)
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

## install: go install to $GOBIN
install:
	go install .

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR)

## help: list targets
help:
	@awk '/^## / { sub(/^## /, ""); printf "  %s\n", $$0 }' $(MAKEFILE_LIST)
