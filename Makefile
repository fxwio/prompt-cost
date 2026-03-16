SHELL := /bin/bash
APP_NAME  := prompt-cost
BUILD_DIR := build
BINARY    := $(BUILD_DIR)/$(APP_NAME)
GO        ?= go

.PHONY: all build test test-integration lint dev dev-down clean

all: build test

build:
	mkdir -p $(BUILD_DIR)
	$(GO) build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/server/main.go

test:
	$(GO) test -race -cover ./internal/...

test-integration:
	$(GO) test -race -tags integration -timeout 120s ./integration/...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)

dev:
	@test -f .env    || (echo "ERROR: .env not found. cp .env.example .env" && exit 1)
	@test -f config.yaml || cp config.example.yaml config.yaml
	docker compose up --build -d
	@echo ""
	@echo "Prompt & Cost Platform: http://localhost:8092"
	@echo ""
	@echo "Logs: docker compose logs -f prompt-cost"

dev-down:
	docker compose down
