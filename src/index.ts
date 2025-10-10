#!/usr/bin/env node

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  ListResourcesRequestSchema,
  ReadResourceRequestSchema,
  ListPromptsRequestSchema,
  GetPromptRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";

// Server configuration
const SERVER_NAME = "keylime-mcp";
const SERVER_VERSION = "0.1.0";

// Create server instance
const server = new Server(
  {
    name: SERVER_NAME,
    version: SERVER_VERSION,
  },
  {
    capabilities: {
      tools: {},
      resources: {},
      prompts: {},
    },
  }
);

// Tool definitions for Keylime operations
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [
      {
        name: "check_agent_status",
        description:
          "Check the status of a Keylime agent. Provides information about whether an agent is registered and its attestation status.",
        inputSchema: {
          type: "object",
          properties: {
            agent_id: {
              type: "string",
              description: "The unique identifier (UUID) of the Keylime agent to check",
            },
            verifier_url: {
              type: "string",
              description: "The URL of the Keylime verifier (default: http://localhost:8881)",
              default: "http://localhost:8881",
            },
          },
          required: ["agent_id"],
        },
      },
      {
        name: "list_agents",
        description:
          "List all registered Keylime agents in the system. Shows agent IDs and basic status information.",
        inputSchema: {
          type: "object",
          properties: {
            verifier_url: {
              type: "string",
              description: "The URL of the Keylime verifier (default: http://localhost:8881)",
              default: "http://localhost:8881",
            },
          },
        },
      },
      {
        name: "get_agent_info",
        description:
          "Get detailed information about a specific Keylime agent including attestation policies, measured boot data, and IMA logs.",
        inputSchema: {
          type: "object",
          properties: {
            agent_id: {
              type: "string",
              description: "The unique identifier (UUID) of the Keylime agent",
            },
            verifier_url: {
              type: "string",
              description: "The URL of the Keylime verifier (default: http://localhost:8881)",
              default: "http://localhost:8881",
            },
          },
          required: ["agent_id"],
        },
      },
      {
        name: "validate_attestation_policy",
        description:
          "Validate an attestation policy file for syntax and semantic correctness. Helps ensure policies are properly formatted.",
        inputSchema: {
          type: "object",
          properties: {
            policy: {
              type: "string",
              description: "The attestation policy content (JSON format)",
            },
          },
          required: ["policy"],
        },
      },
      {
        name: "generate_ima_policy",
        description:
          "Generate an IMA (Integrity Measurement Architecture) policy template for common use cases.",
        inputSchema: {
          type: "object",
          properties: {
            policy_type: {
              type: "string",
              enum: ["basic", "strict", "custom"],
              description: "Type of IMA policy to generate (basic, strict, or custom)",
            },
            allowlist: {
              type: "array",
              items: { type: "string" },
              description: "List of file paths or patterns to include in the allowlist",
            },
          },
          required: ["policy_type"],
        },
      },
    ],
  };
});

// Tool implementation
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  switch (name) {
    case "check_agent_status": {
      const { agent_id, verifier_url = "http://localhost:8881" } = args as {
        agent_id: string;
        verifier_url?: string;
      };

      return {
        content: [
          {
            type: "text",
            text: `Checking agent status for ${agent_id}...\n\nNote: This is a template implementation. To actually check agent status, you would:\n\n1. Send a GET request to ${verifier_url}/v2.1/agents/${agent_id}\n2. Parse the response for operational_state and attestation status\n3. Common states: registered, failed, invalid_quote\n\nExample curl command:\ncurl -X GET "${verifier_url}/v2.1/agents/${agent_id}"`,
          },
        ],
      };
    }

    case "list_agents": {
      const { verifier_url = "http://localhost:8881" } = args as {
        verifier_url?: string;
      };

      return {
        content: [
          {
            type: "text",
            text: `Listing all agents from ${verifier_url}...\n\nNote: This is a template implementation. To actually list agents, you would:\n\n1. Send a GET request to ${verifier_url}/v2.1/agents\n2. Parse the JSON response containing the list of agents\n3. Display agent IDs, operational states, and IP addresses\n\nExample curl command:\ncurl -X GET "${verifier_url}/v2.1/agents"`,
          },
        ],
      };
    }

    case "get_agent_info": {
      const { agent_id, verifier_url = "http://localhost:8881" } = args as {
        agent_id: string;
        verifier_url?: string;
      };

      return {
        content: [
          {
            type: "text",
            text: `Getting detailed information for agent ${agent_id}...\n\nNote: This is a template implementation. To get full agent details, you would:\n\n1. Send a GET request to ${verifier_url}/v2.1/agents/${agent_id}\n2. Parse the detailed response including:\n   - Operational state\n   - Attestation policy\n   - TPM metadata\n   - IMA measurement list\n   - Measured boot policy\n\nExample curl command:\ncurl -X GET "${verifier_url}/v2.1/agents/${agent_id}"`,
          },
        ],
      };
    }

    case "validate_attestation_policy": {
      const { policy } = args as { policy: string };

      try {
        const parsed = JSON.parse(policy);
        const validKeys = ["meta", "allowlist", "excludelist", "ima", "measured_boot"];
        const hasValidStructure = Object.keys(parsed).some((key) =>
          validKeys.includes(key)
        );

        if (hasValidStructure) {
          return {
            content: [
              {
                type: "text",
                text: `✓ Policy validation passed!\n\nThe policy appears to have valid structure with recognized keys.\n\nDetected sections:\n${Object.keys(parsed).map((k) => `  - ${k}`).join("\n")}\n\nNote: For complete validation, use Keylime's built-in policy validation tools.`,
              },
            ],
          };
        } else {
          return {
            content: [
              {
                type: "text",
                text: `⚠ Warning: Policy may be missing expected sections.\n\nFound keys: ${Object.keys(parsed).join(", ")}\nExpected keys: ${validKeys.join(", ")}\n\nPlease verify your policy structure matches Keylime's requirements.`,
              },
            ],
          };
        }
      } catch (error) {
        return {
          content: [
            {
              type: "text",
              text: `✗ Policy validation failed!\n\nError: ${error instanceof Error ? error.message : "Invalid JSON"}\n\nPlease ensure the policy is valid JSON format.`,
            },
          ],
        };
      }
    }

    case "generate_ima_policy": {
      const { policy_type, allowlist = [] } = args as {
        policy_type: string;
        allowlist?: string[];
      };

      let policy: any = {
        meta: {
          version: "1.0",
          generator: "keylime-mcp",
        },
      };

      if (policy_type === "basic") {
        policy.allowlist = [
          "/lib/**",
          "/usr/lib/**",
          "/bin/**",
          "/usr/bin/**",
          "/sbin/**",
          "/usr/sbin/**",
          ...allowlist,
        ];
        policy.excludelist = ["/var/log/**", "/tmp/**"];
      } else if (policy_type === "strict") {
        policy.allowlist = ["/usr/bin/python3", "/usr/lib/python3/**", ...allowlist];
        policy.excludelist = ["/var/**", "/tmp/**", "/home/**"];
        policy.ima = {
          ignored_keyrings: [],
          log_hash_alg: "sha256",
        };
      } else {
        policy.allowlist = allowlist;
        policy.excludelist = [];
      }

      return {
        content: [
          {
            type: "text",
            text: `Generated ${policy_type} IMA policy:\n\n\`\`\`json\n${JSON.stringify(policy, null, 2)}\n\`\`\`\n\nYou can customize this policy further and apply it to your Keylime agent.`,
          },
        ],
      };
    }

    default:
      throw new Error(`Unknown tool: ${name}`);
  }
});

// Resource definitions (documentation and guides)
server.setRequestHandler(ListResourcesRequestSchema, async () => {
  return {
    resources: [
      {
        uri: "keylime://docs/quickstart",
        name: "Keylime Quick Start Guide",
        description: "Getting started with Keylime remote attestation",
        mimeType: "text/plain",
      },
      {
        uri: "keylime://docs/architecture",
        name: "Keylime Architecture Overview",
        description: "Understanding Keylime's architecture and components",
        mimeType: "text/plain",
      },
      {
        uri: "keylime://docs/attestation",
        name: "Attestation Guide",
        description: "How to configure and use attestation policies",
        mimeType: "text/plain",
      },
      {
        uri: "keylime://docs/ima",
        name: "IMA Policy Guide",
        description: "Working with Integrity Measurement Architecture policies",
        mimeType: "text/plain",
      },
    ],
  };
});

// Resource content handler
server.setRequestHandler(ReadResourceRequestSchema, async (request) => {
  const { uri } = request.params;

  switch (uri) {
    case "keylime://docs/quickstart":
      return {
        contents: [
          {
            uri,
            mimeType: "text/plain",
            text: `# Keylime Quick Start Guide

## Overview
Keylime is a remote attestation framework that provides measured boot and runtime integrity monitoring for cloud and edge systems.

## Components
1. **Keylime Agent**: Runs on the machine to be attested
2. **Keylime Verifier**: Validates attestation evidence
3. **Keylime Registrar**: Manages agent enrollment
4. **Keylime Tenant**: CLI tool for managing agents

## Basic Setup
1. Install Keylime on both verifier and agent machines
2. Start the verifier and registrar services
3. Configure and start the agent
4. Use the tenant to add and monitor agents

## Key Commands
- List agents: \`keylime_tenant -c cvlist\`
- Add agent: \`keylime_tenant -c add -u <agent_uuid>\`
- Check status: \`keylime_tenant -c status -u <agent_uuid>\`

For more information, visit: https://keylime.dev`,
          },
        ],
      };

    case "keylime://docs/architecture":
      return {
        contents: [
          {
            uri,
            mimeType: "text/plain",
            text: `# Keylime Architecture Overview

## System Components

### Verifier
- Validates attestation quotes from agents
- Enforces attestation policies
- Monitors agent health
- REST API: http://localhost:8881

### Registrar
- Handles agent enrollment
- Manages agent credentials
- Stores AIK certificates
- REST API: http://localhost:8891

### Agent
- Collects integrity measurements
- Generates attestation quotes using TPM
- Reports to verifier
- Executes payload scripts
- REST API: http://localhost:9002

### Tenant (CLI)
- Command-line interface for administrators
- Manages agents and policies
- Queries system status

## Communication Flow
1. Agent registers with Registrar
2. Tenant adds agent to Verifier
3. Verifier requests attestation from Agent
4. Agent generates TPM quote and sends to Verifier
5. Verifier validates quote against policy
6. Process repeats periodically

## Security Features
- TPM-based attestation
- Measured boot validation
- Runtime integrity monitoring (IMA)
- Encrypted payloads
- mTLS communication`,
          },
        ],
      };

    case "keylime://docs/attestation":
      return {
        contents: [
          {
            uri,
            mimeType: "text/plain",
            text: `# Attestation Guide

## What is Attestation?
Remote attestation verifies that a system is in a known good state by checking:
- Boot process integrity (measured boot)
- Runtime file integrity (IMA)
- TPM PCR values

## Attestation Policies

### Basic Policy Structure
\`\`\`json
{
  "meta": {
    "version": "1.0"
  },
  "allowlist": [
    "/lib/**",
    "/usr/lib/**"
  ],
  "excludelist": [
    "/tmp/**"
  ]
}
\`\`\`

### Policy Types
1. **Allowlist**: Files that are expected to be measured
2. **Excludelist**: Files to ignore in measurements
3. **Measured Boot**: PCR values for boot components
4. **IMA Policy**: Runtime integrity rules

## Applying Policies
Use the tenant CLI to apply policies:
\`\`\`bash
keylime_tenant -c add -u <agent_id> \\
  --allowlist allowlist.txt \\
  --mb_refstate mb_policy.json
\`\`\`

## Monitoring Attestation
Check agent status regularly:
- GREEN: All checks passing
- YELLOW: Warning state
- RED: Attestation failure

Failed attestation triggers revocation actions.`,
          },
        ],
      };

    case "keylime://docs/ima":
      return {
        contents: [
          {
            uri,
            mimeType: "text/plain",
            text: `# IMA Policy Guide

## Integrity Measurement Architecture (IMA)

IMA is a Linux kernel subsystem that measures file integrity at runtime.

## IMA Policy Components

### Allowlist
Defines which files should be measured and their expected hashes.

Example:
\`\`\`
/usr/bin/python3 sha256:abc123...
/lib/x86_64-linux-gnu/** 
\`\`\`

### Excludelist
Files to exclude from measurement (logs, temporary files).

Example:
\`\`\`
/var/log/**
/tmp/**
/proc/**
\`\`\`

## Policy Formats

### JSON Format
\`\`\`json
{
  "allowlist": [
    "/usr/bin/python3",
    "/usr/lib/python3/**"
  ],
  "excludelist": [
    "/var/log/**"
  ],
  "ima": {
    "log_hash_alg": "sha256"
  }
}
\`\`\`

## Best Practices
1. Start with a broad allowlist, then refine
2. Always exclude log and temporary directories
3. Use wildcards for library directories
4. Test policies before production deployment
5. Monitor IMA logs for unexpected measurements

## Generating Allowlists
Use IMA measurement logs to generate allowlists:
\`\`\`bash
cat /sys/kernel/security/ima/ascii_runtime_measurements
\`\`\`

Process these measurements to create your policy.`,
          },
        ],
      };

    default:
      throw new Error(`Unknown resource: ${uri}`);
  }
});

// Prompt definitions for common tasks
server.setRequestHandler(ListPromptsRequestSchema, async () => {
  return {
    prompts: [
      {
        name: "setup_new_agent",
        description: "Guide for setting up a new Keylime agent",
      },
      {
        name: "troubleshoot_attestation",
        description: "Help troubleshooting attestation failures",
      },
      {
        name: "create_ima_policy",
        description: "Interactive guide for creating IMA policies",
      },
    ],
  };
});

// Prompt content handler
server.setRequestHandler(GetPromptRequestSchema, async (request) => {
  const { name } = request.params;

  switch (name) {
    case "setup_new_agent":
      return {
        messages: [
          {
            role: "user",
            content: {
              type: "text",
              text: "I need help setting up a new Keylime agent",
            },
          },
          {
            role: "assistant",
            content: {
              type: "text",
              text: `I'll help you set up a new Keylime agent. Here's the process:

1. **Install Keylime Agent**
   - On Debian/Ubuntu: \`apt-get install keylime-agent\`
   - On RHEL/Fedora: \`dnf install keylime-agent\`

2. **Configure the Agent**
   Edit \`/etc/keylime/agent.conf\`:
   - Set verifier and registrar URLs
   - Configure TPM settings
   - Set agent UUID (or let it auto-generate)

3. **Start the Agent Service**
   \`\`\`bash
   systemctl start keylime_agent
   systemctl enable keylime_agent
   \`\`\`

4. **Register with Verifier**
   On the tenant machine:
   \`\`\`bash
   keylime_tenant -c add -u <agent_uuid> -v <verifier_url>
   \`\`\`

5. **Verify Registration**
   \`\`\`bash
   keylime_tenant -c status -u <agent_uuid>
   \`\`\`

Would you like help with any specific step?`,
            },
          },
        ],
      };

    case "troubleshoot_attestation":
      return {
        messages: [
          {
            role: "user",
            content: {
              type: "text",
              text: "My Keylime agent is failing attestation",
            },
          },
          {
            role: "assistant",
            content: {
              type: "text",
              text: `Let's troubleshoot the attestation failure. Here's a systematic approach:

1. **Check Agent Status**
   \`\`\`bash
   keylime_tenant -c status -u <agent_uuid>
   \`\`\`
   Look for the operational_state field.

2. **Review Verifier Logs**
   \`\`\`bash
   journalctl -u keylime_verifier -n 100
   \`\`\`
   Look for specific error messages.

3. **Common Issues:**

   **TPM Quote Validation Failed**
   - Verify TPM is functioning: \`tpm2_pcrread\`
   - Check PCR values match expected
   - Ensure time synchronization (NTP)

   **IMA Measurement Mismatch**
   - Review IMA log: \`cat /sys/kernel/security/ima/ascii_runtime_measurements\`
   - Check if unexpected files were modified
   - Verify allowlist is current

   **Measured Boot Failure**
   - Check boot logs: \`dmesg | grep -i tpm\`
   - Verify UEFI Secure Boot status
   - Review measured boot policy

4. **Reset and Re-register**
   If needed, remove and re-add the agent:
   \`\`\`bash
   keylime_tenant -c delete -u <agent_uuid>
   keylime_tenant -c add -u <agent_uuid>
   \`\`\`

What specific error are you seeing?`,
            },
          },
        ],
      };

    case "create_ima_policy":
      return {
        messages: [
          {
            role: "user",
            content: {
              type: "text",
              text: "Help me create an IMA policy for my application",
            },
          },
          {
            role: "assistant",
            content: {
              type: "text",
              text: `I'll help you create an IMA policy. Let's start with some questions:

1. **What type of system is this?**
   - Web server
   - Database server
   - Container host
   - General purpose
   - Other

2. **Security Level Needed?**
   - Basic (allow common system files)
   - Strict (allowlist specific applications only)
   - Custom (you define everything)

3. **Key Directories to Monitor?**
   Examples:
   - \`/usr/bin/\` - System binaries
   - \`/usr/lib/\` - System libraries
   - \`/opt/myapp/\` - Your application
   - \`/etc/\` - Configuration files

4. **Directories to Exclude?**
   Common exclusions:
   - \`/var/log/\` - Log files
   - \`/tmp/\` - Temporary files
   - \`/var/cache/\` - Cache files

You can use the \`generate_ima_policy\` tool to create a template:
- Basic policy: Common system paths
- Strict policy: Minimal allowlist
- Custom policy: Your specific paths

Would you like me to generate a template based on your needs?`,
            },
          },
        ],
      };

    default:
      throw new Error(`Unknown prompt: ${name}`);
  }
});

// Start the server
async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("Keylime MCP Server running on stdio");
}

main().catch((error) => {
  console.error("Fatal error in main():", error);
  process.exit(1);
});
