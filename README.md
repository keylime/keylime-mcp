# keylime-mcp

A Model Context Protocol (MCP) server for [Keylime](https://keylime.dev), the remote attestation framework for cloud and edge systems.

## Overview

This MCP server provides AI assistants with tools and knowledge to work with Keylime, including:

- **Tools** for managing Keylime agents and attestation
- **Resources** with documentation and guides
- **Prompts** for common Keylime tasks and troubleshooting

## Features

### Tools

- `check_agent_status` - Check the status of a Keylime agent
- `list_agents` - List all registered agents
- `get_agent_info` - Get detailed agent information
- `validate_attestation_policy` - Validate policy syntax
- `generate_ima_policy` - Generate IMA policy templates

### Resources

- Quick Start Guide - Getting started with Keylime
- Architecture Overview - Understanding Keylime components
- Attestation Guide - Working with attestation policies
- IMA Policy Guide - Integrity Measurement Architecture policies

### Prompts

- `setup_new_agent` - Interactive agent setup guide
- `troubleshoot_attestation` - Attestation failure troubleshooting
- `create_ima_policy` - IMA policy creation wizard

## Installation

```bash
npm install
npm run build
```

## Usage

### With Claude Desktop

Add to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "keylime": {
      "command": "node",
      "args": ["/path/to/keylime-mcp/dist/index.js"]
    }
  }
}
```

### With Other MCP Clients

The server communicates via stdio and can be used with any MCP-compatible client:

```bash
node dist/index.js
```

## Development

```bash
# Install dependencies
npm install

# Build TypeScript
npm run build

# Run in development mode
npm run dev

# Watch mode for development
npm run watch
```

## About Keylime

[Keylime](https://keylime.dev) is an open-source remote attestation framework that provides:

- **Measured Boot** verification via TPM
- **Runtime Integrity** monitoring with IMA
- **Secure Enrollment** and key management
- **Policy-based Attestation** with automated responses

## Requirements

This MCP server is a helper tool for working with Keylime. To actually interact with a Keylime deployment, you need:

- A running Keylime verifier and registrar
- Keylime agents to monitor
- Network access to the Keylime API endpoints

## Contributing

Contributions are welcome! This is an experimental project to explore MCP integration with Keylime.

## License

Apache-2.0

## Resources

- [Keylime Documentation](https://keylime-docs.readthedocs.io/)
- [Keylime GitHub](https://github.com/keylime/keylime)
- [Model Context Protocol](https://modelcontextprotocol.io/)