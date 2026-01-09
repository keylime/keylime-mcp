.PHONY: help build up down logs clean ps

PODMAN := $(shell command -v podman 2>/dev/null)

ifeq ($(PODMAN),)
$(error Podman not found. Install: sudo dnf install podman podman-compose)
endif

help:
	@echo "Keylime MCP"
	@echo "  make server  - Build MCP server binary"
	@echo "  make client  - Build web client binary"
	@echo "  make run     - Run web client locally"

.env:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env from .env.example"; \
	fi

server:
	go build -o bin/server cmd/server/main.go

client: server
	go build -o bin/client cmd/client/main.go

run: .env server client
	cd bin/ && ./client


# TODO Podman
