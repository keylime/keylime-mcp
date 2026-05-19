# Keylime MCP

MCP server for [Keylime](https://keylime.dev) remote attestation.

## Requirements

- Running [Keylime Verifier](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html/security_hardening/assembly_ensuring-system-integrity-with-keylime_security-hardening#configuring-keylime-verifier_assembly_ensuring-system-integrity-with-keylime) and [Registrar](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html/security_hardening/assembly_ensuring-system-integrity-with-keylime_security-hardening#configuring-keylime-registrar_assembly_ensuring-system-integrity-with-keylime)
- [Keylime agents](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html/security_hardening/assembly_ensuring-system-integrity-with-keylime_security-hardening#configuring-keylime-agent_assembly_ensuring-system-integrity-with-keylime) to monitor
- Network access to Keylime API endpoints
- **MCP client** (Claude Code, Claude Desktop, Cline, etc.)

## Usage

### Option 1: MCP Client

Build the server:
```bash
make build-server
```

Add to your MCP client config (e.g., `~/.config/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "keylime": {
      "command": "/full/path/to/keylime-mcp/bin/server",
      "env": {
        "KEYLIME_CERT_DIR": "/full/path/to/keylime/certs/dir"
      }
    }
  }
}
```

Replace paths with your actual locations. Certs are typically in `/var/lib/keylime/cv_ca` (need read permissions — run `make setup-certs`).

### Option 2: Web UI

```bash
make run
```
Access at http://localhost:3000

## Configuration

Copy `.env.example` to `.env` and adjust:

| Variable | Default | Description |
|---|---|---|
| `KEYLIME_VERIFIER_URL` | `https://localhost:8881` | Verifier API endpoint |
| `KEYLIME_REGISTRAR_URL` | `https://localhost:8891` | Registrar API endpoint |
| `KEYLIME_API_VERSION` | `v2.5` | Keylime REST API version |
| `KEYLIME_CERT_DIR` | `/var/lib/keylime/cv_ca` | mTLS certificate directory |
| `KEYLIME_TLS_ENABLED` | `true` | Enable mTLS for Keylime communication |
| `KEYLIME_TLS_SERVER_NAME` | `localhost` | Expected server name in Keylime certificate SAN |
| `ANTHROPIC_API_KEY` | -- | Required for Claude provider |
| `OLLAMA_URL` | `http://localhost:11434` | Ollama API endpoint (for local LLM) |
| `OLLAMA_MODEL` | -- | Ollama model name (e.g., `qwen2.5`) |
| `MASKING_ENABLED` | `true` | Mask sensitive data before sending to LLM |
| `PORT` | `3000` | Web UI port |

## Commands

Control over the project is managed by a Makefile. Since this project works as a local controller, the Makefile handles these tasks perfectly and allows the user to manage the whole system with the following commands:

| Command | Description |
|---|---|
| `make install` | Should be run on first setup. Runs `setup-certs`, `check-deps`, creates `.env` from `.env.example`, and builds everything |
| `make build-server` | Compiles only the MCP server binary |
| `make build` | Compiles the whole project (client and server) |
| `make run` | Compiles and runs the whole project |
| `make start` | Runs the project without compiling (uses pre-built binaries) |
| `make setup-certs` | Grants read access to Keylime certificates and persists across reboots via a systemd service |
| `make setup-certs-session` | Same but only for the current session (does not survive reboot) |
| `make check-deps` | Checks all dependencies for running the project |
| `make clean` | Removes compiled binaries and the systemd service created by `setup-certs` |
| `make test` | Runs unit tests |
| `make test-race` | Runs unit tests with the `-race` flag (detects data races) |
| `make test-e2e` | Submits end-to-end tests to Testing Farm. Requires Red Hat VPN access |

## Testing

### Unit tests

```bash
make test          # run unit tests
make test-race     # run with race detector
```

### E2E tests (Testing Farm)

E2E tests run on [Testing Farm](https://docs.testing-farm.io/) against a real Keylime deployment with emulated TPM. Triggered automatically on PRs via [Packit](https://packit.dev/).

```bash
make test-e2e      # requires Testing Farm API token + Red Hat VPN
```

TMT plans in `e2e/plans/`: `keylime-mcp-server`, `keylime-mcp-client`, `keylime-mcp-main`.

## Further Reading

- [Architecture](doc/architecture.md)
- [MCP Tools Reference](doc/tools.md)
- [Keylime Documentation](https://keylime-docs.readthedocs.io/)
- [Keylime GitHub](https://github.com/keylime/keylime)
- [Model Context Protocol](https://modelcontextprotocol.io/)

## Contributing

Contributions welcome.

## License

Apache-2.0
