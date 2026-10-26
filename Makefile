SHELL := /bin/bash
GO ?= go
NPM ?= npm
BIN_DIR := bin
BINARY := $(BIN_DIR)/aigw
WEB_DIR := web

.PHONY: all build web web-install web-check web-dev run dev test fmt vet lint clean container help

all: build

help:
	@echo "Targets:"
	@echo "  make web          build the SvelteKit frontend into web/build"
	@echo "  make build        build the Go binary and frontend"
	@echo "  make run          run the gateway (expects configs/config.yml or AIGW_* env)"
	@echo "  make dev          run backend and frontend dev servers in parallel"
	@echo "  make test         run all Go tests"
	@echo "  make web-check    run svelte-check on the frontend"
	@echo "  make fmt          format Go sources"
	@echo "  make container    build the container image"
	@echo "  make clean        remove build artifacts"

web-install:
	cd $(WEB_DIR) && $(NPM) ci

web-check: web-install
	cd $(WEB_DIR) && $(NPM) run check

web: web-install
	cd $(WEB_DIR) && $(NPM) run build

build: web
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w" -o $(BINARY) ./cmd/aigw

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint: fmt vet

run:
	$(GO) run ./cmd/aigw -config $${AIGW_CONFIG:-configs/config.yml}

dev:
	@trap 'kill 0' SIGINT; \
	($(GO) run ./cmd/aigw -config $${AIGW_CONFIG:-configs/config.example.yml}) & \
	(cd $(WEB_DIR) && $(NPM) run dev -- --host 0.0.0.0); \
	wait

container:
	docker build -f Containerfile -t aigw:latest .

clean:
	rm -rf $(BIN_DIR) $(WEB_DIR)/build $(WEB_DIR)/.svelte-kit $(WEB_DIR)/node_modules
