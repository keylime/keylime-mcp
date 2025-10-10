# Usage Examples

This document provides examples of how to use the Keylime MCP server with AI assistants.

## Getting Started

Once the server is configured in your MCP client (e.g., Claude Desktop), you can interact with it naturally using conversational queries.

## Example Queries

### Checking Agent Status

```
Can you check the status of Keylime agent d432fbb3-d2f1-4a97-9ef0-52e2d09d381b?
```

The assistant will use the `check_agent_status` tool to check the agent's status.

### Listing All Agents

```
Show me all the registered Keylime agents
```

The assistant will use the `list_agents` tool to retrieve all agents.

### Getting Detailed Agent Information

```
I need detailed information about agent d432fbb3-d2f1-4a97-9ef0-52e2d09d381b
```

The assistant will use the `get_agent_info` tool.

### Validating Attestation Policies

```
Can you validate this attestation policy:
{
  "allowlist": ["/usr/bin/**", "/lib/**"],
  "excludelist": ["/tmp/**"]
}
```

The assistant will use the `validate_attestation_policy` tool to check the policy syntax.

### Generating IMA Policies

```
Generate a basic IMA policy for my web server
```

The assistant will use the `generate_ima_policy` tool to create a template.

## Using Resources

### Learning About Keylime

```
What is Keylime's architecture?
```

The assistant can access the architecture documentation resource.

```
How do I set up attestation policies?
```

The assistant can access the attestation guide resource.

### IMA Policies

```
Explain how IMA policies work in Keylime
```

The assistant can access the IMA policy guide resource.

## Using Prompts

### Setting Up a New Agent

```
Help me set up a new Keylime agent
```

This will activate the `setup_new_agent` prompt for an interactive guide.

### Troubleshooting

```
My Keylime agent is failing attestation, what should I check?
```

This will activate the `troubleshoot_attestation` prompt for debugging help.

### Creating IMA Policies

```
I need to create an IMA policy for my application
```

This will activate the `create_ima_policy` prompt for an interactive policy creation wizard.

## Advanced Usage

### Custom Verifier URL

```
Check agent status for d432fbb3-d2f1-4a97-9ef0-52e2d09d381b using verifier at https://my-verifier.example.com:8881
```

You can specify custom verifier URLs when needed.

### Policy Generation with Custom Paths

```
Generate a strict IMA policy that includes /opt/myapp/** in the allowlist
```

You can customize the generated policies with specific paths.

## Tips

1. **Be Specific**: Include agent UUIDs when checking specific agents
2. **Ask for Explanations**: The assistant can explain Keylime concepts using the resources
3. **Iterate**: Start with generated policies and refine them based on your needs
4. **Use Context**: The assistant remembers context within a conversation

## Note

This MCP server provides **templates and guidance** for working with Keylime. For actual API integration, you would need to implement HTTP requests to your Keylime verifier and registrar endpoints. The tools currently show example commands and guidance rather than making actual API calls.
