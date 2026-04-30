.PHONY: help build build-server run start check-deps setup-certs install clean test test-race test-functional

help:
	@echo "Keylime MCP"
	@echo ""
	@echo "Setup:"
	@echo "  make install      - Full setup (check deps, env, certs, build)"
	@echo "  make check-deps   - Verify Go is installed and certs are readable"
	@echo "  make setup-certs  - Grant read access to Keylime certs (requires sudo)"
	@echo ""
	@echo "Build & Run:"
	@echo "  make build-server - Build MCP server binary"
	@echo "  make build        - Build everything (server + client)"
	@echo "  make run          - Build and run"
	@echo "  make start        - Run pre-built binary (no compilation)"
	@echo ""
	@echo "Tests:"
	@echo "  make test              - Run server tests"
	@echo "  make test-race         - Run server tests with race detector"
	@echo "  make test-functional   - Run functional tests via Testing Farm"

.env:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env from .env.example"; \
	fi

build-server:
	go build -o bin/server cmd/server/main.go

build: build-server
	go build -o bin/client cmd/client/main.go

run: .env build
	cd bin/ && ./client

start:
	cd bin/ && ./client

KEYLIME_CERT_DIR := /var/lib/keylime/cv_ca
CERT_FILES := cacert.crt client-cert.crt client-private.pem

setup-certs:
	@echo "Granting read access to Keylime certificates for user '$(USER)'..."
	@sudo setfacl -m u:$(USER):rx /var/lib/keylime
	@sudo setfacl -m u:$(USER):rx $(KEYLIME_CERT_DIR)
	@for f in $(CERT_FILES); do \
		sudo setfacl -m u:$(USER):r "$(KEYLIME_CERT_DIR)/$$f"; \
	done
	@echo "Done. Certificate access granted."

check-deps:
	@echo "Checking dependencies..."
	@command -v go >/dev/null 2>&1 || { echo "Go not found. Install: https://go.dev/dl/"; exit 1; }
	@echo "  Go: $$(go version)"
	@for f in $(CERT_FILES); do \
		if [ ! -r "$(KEYLIME_CERT_DIR)/$$f" ]; then \
			echo "  Certificate not readable: $(KEYLIME_CERT_DIR)/$$f"; \
			echo "  Run 'make setup-certs' to fix."; \
			exit 1; \
		fi; \
	done
	@echo "  Certs: OK"
	@echo "All dependencies satisfied."

install: setup-certs check-deps .env build
	@echo "Installation complete. Run 'make run' or 'make start'."

clean:
	rm -rf bin/*

# Tests

test:
	go test ./internal/keylime/... ./internal/mcptools/... ./cmd/server/... -count=1

test-race:
	go test ./internal/keylime/... ./internal/mcptools/... ./cmd/server/... -race -count=1

test-functional:
	go test -v -count=1 -timeout=600s -tags=functional ./functional-tests/...
