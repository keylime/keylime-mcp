# Keylime MCP

Model Context Protocol interface for Keylime.

## Quick Start

### With Docker

```bash
make build
make up
```

- Frontend: http://localhost:3000
- Backend: http://localhost:8080/

### Local Development

**Backend:**
```bash
cd backend
go run main.go
```

**Frontend:**
```bash
cd frontend
pnpm dev
```

## Commands

- `make build` - Build containers
- `make up` - Start containers
- `make down` - Stop containers
- `make logs` - View logs
- `make clean` - Remove everything

## Stack

- **Backend**: Go 1.23
- **Frontend**: React + TypeScript + Vite + Tailwind + shadcn/ui
- **Container**: Docker/Podman

