# Keylime MCP

A Model Context Protocol (MCP) server for [Keylime](https://keylime.dev), the remote attestation framework for cloud and edge systems.

## Requirements

This MCP server is a helper tool for working with Keylime. To actually interact with a Keylime deployment, you need:

- A running [Keylime verifier](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html/security_hardening/assembly_ensuring-system-integrity-with-keylime_security-hardening#configuring-keylime-verifier_assembly_ensuring-system-integrity-with-keylime) and [Keylime registrar](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html/security_hardening/assembly_ensuring-system-integrity-with-keylime_security-hardening#configuring-keylime-registrar_assembly_ensuring-system-integrity-with-keylime)
- [Keylime agents](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html/security_hardening/assembly_ensuring-system-integrity-with-keylime_security-hardening#configuring-keylime-agent_assembly_ensuring-system-integrity-with-keylime) to monitor
- Network access to the Keylime API endpoints
- [Podman](https://podman.io/getting-started/installation) must be installed on your system.

## Quick Start

```bash
make build
make up
```
Access at http://localhost:3000

## Development

Run locally without containers:

```bash
# Backend
cd backend && go run main.go

# Frontend
cd frontend && pnpm dev
```

## Commands

- `make build` - Build containers
- `make up` - Start containers
- `make down` - Stop containers  
- `make logs` - View logs
- `make clean` - Remove everything
- `make ps` - List containers
- `make help` - Show all commands

## Stack

- **Backend**: Go 1.23
- **Frontend**: React + TypeScript + Vite + Tailwind + shadcn/ui
- **Container**: Podman

## About Keylime

[Keylime](https://keylime.dev) is an open-source remote attestation framework that provides:

- **Measured Boot** verification via TPM
- **Runtime Integrity** monitoring with IMA
- **Secure Enrollment** and key management
- **Policy-based Attestation** with automated responses

## Contributing

Contributions are welcome! This is an experimental project to explore MCP integration with Keylime.

## License

Apache-2.0

## Resources

- [Keylime Documentation](https://keylime-docs.readthedocs.io/)
- [Keylime GitHub](https://github.com/keylime/keylime)
- [Model Context Protocol](https://modelcontextprotocol.io/)