.PHONY: help build up down logs clean ps

PODMAN := $(shell command -v podman 2>/dev/null)

ifeq ($(PODMAN),)
$(error Podman not found. Install: sudo dnf install podman podman-compose)
endif

help:
	@echo "Keylime MCP (Podman)"
	@echo "  make build  - Build containers"
	@echo "  make up     - Start containers"
	@echo "  make down   - Stop containers"
	@echo "  make logs   - View logs"
	@echo "  make ps     - List containers"
	@echo "  make clean  - Remove all"

build:
	podman-compose -f compose.yml build

up:
	podman-compose -f compose.yml up -d
	@echo "http://localhost:3000"

down:
	podman-compose -f compose.yml down

logs:
	podman-compose -f compose.yml logs -f

ps:
	podman ps -a

clean:
	podman-compose -f compose.yml down -v
	podman system prune -f
