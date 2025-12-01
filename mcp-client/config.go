package main

import "github.com/anthropics/anthropic-sdk-go"

const (
	defaultServerPath = "../backend/server"
	mcpClientName     = "mcp-client"
	mcpClientVersion  = "v1.0.0"

	claudeModel   = anthropic.ModelClaude3_5HaikuLatest
	maxTokens     = 2048
	maxAgentTurns = 5

	systemPrompt = `You are an AI assistant with access to Keylime system management tools. Your goal is to help users manage and monitor their Keylime infrastructure.

You have a maximum of 5 conversation turns to complete the task. When given a task:
1. Break it down into steps if needed
2. Use available tools to gather information and take actions
3. Chain multiple tool calls together to accomplish complex tasks
4. Provide clear explanations of what you're doing and what you found
5. If you encounter failures, investigate and suggest solutions
6. Work efficiently to complete tasks within the turn limit`
)

type Config struct {
	AnthropicAPIKey string
	MCPServerPath   string
}

func NewConfig() (*Config, error) {
	apiKey := getEnvOrDefault("ANTHROPIC_API_KEY", "")
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	serverPath := getEnvOrDefault("MCP_SERVER_PATH", defaultServerPath)

	return &Config{
		AnthropicAPIKey: apiKey,
		MCPServerPath:   serverPath,
	}, nil
}
