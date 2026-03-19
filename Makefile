BINARY := relayshell
GOVERNOR_CMD := ./cmd/governor
GOLANGCI_LINT ?= $(shell command -v golangci-lint)

ifeq ($(GOLANGCI_LINT),)
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
endif

.PHONY: all build run test lint fmt tidy install-tools build-codex-image

all: build

build:
	go build -o $(BINARY) $(GOVERNOR_CMD)

run:
	go run $(GOVERNOR_CMD)

test:
	go test ./...

lint:
	$(GOLANGCI_LINT) run

install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

fmt:
	gofmt -w ./cmd ./internal

tidy:
	go mod tidy

build-codex-image:
	docker build -f Dockerfile.codex -t relayshell-codex:latest .
