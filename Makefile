.PHONY: help build up down logs clean dev-backend dev-frontend

# Detect container runtime (Podman first, then Docker)
PODMAN := $(shell command -v podman 2>/dev/null)
DOCKER := $(shell command -v docker 2>/dev/null)
COMPOSE := $(if $(PODMAN),podman-compose,$(if $(DOCKER),docker compose,))

ifeq ($(COMPOSE),)
$(error No container runtime found. Please install podman or docker)
endif

help:
	@echo "Keylime MCP Commands:"
	@echo "  make build  - Build containers (using $(COMPOSE))"
	@echo "  make up     - Start containers"
	@echo "  make down   - Stop containers"
	@echo "  make logs   - View logs"
	@echo "  make clean  - Remove containers and volumes"

build:
	$(COMPOSE) build

up:
	$(COMPOSE) up -d
	@echo "Started on http://localhost:3000"

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

clean:
	$(COMPOSE) down -v

