BINARY := relayshell
GOVERNOR_CMD := ./cmd/governor
ENV_FILE ?= .env
GOLANGCI_LINT ?= $(shell command -v golangci-lint)

ifeq ($(GOLANGCI_LINT),)
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
endif

.PHONY: all build run test lint fmt tidy install-tools build-codex-image build-copilot-image tuwunel-up tuwunel-down tuwunel-logs governor-run matrix-bootstrap dev-run

all: build

build:
	go build -o $(BINARY) $(GOVERNOR_CMD)

run:
	go run $(GOVERNOR_CMD)

governor-run:
	@set -e; \
	if [ ! -f "$(ENV_FILE)" ]; then \
		echo "Missing $(ENV_FILE). Copy .env.example to .env and fill values."; \
		exit 1; \
	fi; \
	bash ./scripts/run_governor.sh "$(ENV_FILE)"

matrix-bootstrap:
	@set -e; \
	if [ ! -f "$(ENV_FILE)" ]; then \
		echo "Missing $(ENV_FILE). Copy .env.example to .env and fill values."; \
		exit 1; \
	fi; \
	python3 ./scripts/bootstrap_matrix.py "$(ENV_FILE)"

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

build-copilot-image:
	docker build -f Dockerfile.copilot -t relayshell-copilot:latest .

tuwunel-up:
	@set -e; \
	if [ ! -f "$(ENV_FILE)" ]; then \
		echo "Missing $(ENV_FILE). Copy .env.example to .env and fill values."; \
		exit 1; \
	fi; \
	docker compose --env-file $(ENV_FILE) up -d --build tuwunel

tuwunel-down:
	@set -e; \
	if [ ! -f "$(ENV_FILE)" ]; then \
		echo "Missing $(ENV_FILE). Copy .env.example to .env and fill values."; \
		exit 1; \
	fi; \
	docker compose --env-file $(ENV_FILE) down

tuwunel-logs:
	@set -e; \
	if [ ! -f "$(ENV_FILE)" ]; then \
		echo "Missing $(ENV_FILE). Copy .env.example to .env and fill values."; \
		exit 1; \
	fi; \
	docker compose --env-file $(ENV_FILE) logs -f tuwunel

dev-run: tuwunel-up build-codex-image build-copilot-image matrix-bootstrap governor-run
