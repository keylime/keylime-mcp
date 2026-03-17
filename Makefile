.PHONY: help build up down logs clean ps setup-certs

PODMAN := $(shell command -v podman 2>/dev/null)

ifeq ($(PODMAN),)
$(error Podman not found. Install: sudo dnf install podman podman-compose)
endif

help:
	@echo "Keylime MCP"
	@echo "  make server      - Build MCP server binary"
	@echo "  make client      - Build web client binary"
	@echo "  make run         - Run web client locally"
	@echo "  make setup-certs - Grant read access to Keylime certs (requires sudo)"

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

KEYLIME_CERT_DIR := /var/lib/keylime/cv_ca
CERT_FILES := cacert.crt client-cert.crt client-private.pem

setup-certs:
	@echo "Granting read access to Keylime certificates for user '$(USER)'..."
	@sudo setfacl -m u:$(USER):rx /var/lib/keylime
	@sudo setfacl -m u:$(USER):rx $(KEYLIME_CERT_DIR)
	@for f in $(CERT_FILES); do \
		sudo setfacl -m u:$(USER):r $(KEYLIME_CERT_DIR)/$$f; \
	done
	@echo "Done. Certificate access granted."

# TODO Podman
