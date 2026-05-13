.PHONY: help build build-server run start check-deps setup-certs setup-certs-session install clean test test-race test-e2e

help:
	@echo "Keylime MCP"
	@echo ""
	@echo "Setup:"
	@echo "  make install      - Full setup (check deps, env, certs, build)"
	@echo "  make check-deps   - Verify Go is installed and certs are readable"
	@echo "  make setup-certs          - Grant read access to Keylime certs, persists across reboots (requires sudo)"
	@echo "  make setup-certs-session  - Same but only for current session (does not survive reboot)"
	@echo ""
	@echo "Build & Run:"
	@echo "  make build-server - Build MCP server binary"
	@echo "  make build        - Build everything (server + client)"
	@echo "  make run          - Build and run"
	@echo "  make start        - Run pre-built binary (no compilation)"
	@echo ""
	@echo "Tests:"
	@echo "  make test              - Run unit tests"
	@echo "  make test-race         - Run unit tests with race detector"
	@echo "  make test-e2e     		- Submit e2e tests to Testing Farm (requires testing-farm + credentials)"

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

SYSTEMD_SERVICE := /etc/systemd/system/keylime-mcp-certs.service

setup-certs: setup-certs-session
	@echo "Persisting certificate access across reboots..."
	@printf '%s\n' \
		"[Unit]" \
		"Description=Grant certificate access for Keylime MCP" \
		"After=systemd-tmpfiles-setup.service" \
		"[Service]" \
		"Type=oneshot" \
		$(foreach f,$(CERT_FILES),"ExecStart=/usr/bin/setfacl -m u:$(USER):r $(KEYLIME_CERT_DIR)/$(f)") \
		"ExecStart=/usr/bin/setfacl -m u:$(USER):rx /var/lib/keylime" \
		"ExecStart=/usr/bin/setfacl -m u:$(USER):rx $(KEYLIME_CERT_DIR)" \
		"[Install]" \
		"WantedBy=multi-user.target" \
		| sudo tee $(SYSTEMD_SERVICE) > /dev/null
	@sudo systemctl daemon-reload
	@sudo systemctl enable keylime-mcp-certs.service
	@echo "Done. Certificate access granted and will persist across reboots."

setup-certs-session:
	@echo "Granting read access to Keylime certificates for user '$(USER)' (session only)..."
	@sudo setfacl -m u:$(USER):rx /var/lib/keylime
	@sudo setfacl -m u:$(USER):rx $(KEYLIME_CERT_DIR)
	@for f in $(CERT_FILES); do \
		sudo setfacl -m u:$(USER):r "$(KEYLIME_CERT_DIR)/$$f"; \
	done
	@echo "Done. Certificate access granted (will not persist across reboots)."

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
	@if [ -f $(SYSTEMD_SERVICE) ]; then \
		sudo systemctl disable keylime-mcp-certs.service 2>/dev/null; \
		sudo rm -f $(SYSTEMD_SERVICE); \
		sudo systemctl daemon-reload; \
		echo "Removed keylime-mcp-certs.service"; \
	fi

# Tests

test:
	go test ./internal/... ./cmd/... -count=1

test-race:
	go test ./internal/... ./cmd/... -race -count=1

test-e2e:
	testing-farm request \
		--compose Fedora-Rawhide \
		--plan 'e2e/plans/keylime-mcp-main' \
		--arch x86_64,aarch64,ppc64le,s390x
