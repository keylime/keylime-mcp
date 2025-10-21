.PHONY: help build up down logs clean

help:
	@echo "Available commands:"
	@echo "  make build  - Build containers"
	@echo "  make up     - Start containers"
	@echo "  make down   - Stop containers"
	@echo "  make logs   - View logs"
	@echo "  make clean  - Remove containers and volumes"

build:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

clean:
	docker compose down -v

