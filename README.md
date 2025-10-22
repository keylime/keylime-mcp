# Keylime MCP

Model Context Protocol interface for Keylime.

## Prerequisites

- Podman
- Keylime

## Quick Start

```bash
make build
make up
```

Access at http://localhost:3000

The Makefile automatically works with Docker or Podman.

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
- `make ps` - List containers"
- `make help` - Show all commands

## Stack

- **Backend**: Go 1.23
- **Frontend**: React + TypeScript + Vite + Tailwind + shadcn/ui
- **Container**: Docker/Podman

