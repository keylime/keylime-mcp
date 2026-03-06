package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/keylime/keylime-mcp/internal/agent"
	"github.com/keylime/keylime-mcp/internal/web"
)

const (
	defaultServerPath = "./server"
	defaultPort       = "3000"
	defaultOllamaURL  = "http://localhost:11434"
	envFileLocation   = "./../.env"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\nShutting down gracefully...")
		cancel()
	}()

	if err := godotenv.Load(envFileLocation); err != nil {
		log.Printf("Warning: .env file not loaded: %v", err)
	}

	serverPath := os.Getenv("MCP_SERVER_PATH")
	if serverPath == "" {
		serverPath = defaultServerPath
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	if _, err := os.Stat(serverPath); os.IsNotExist(err) { // #nosec G703 -- serverPath from env/default, not user input
		log.Printf("Warning: MCP server not found at %s", serverPath)
		log.Printf("Build the server first: go build -o bin/server cmd/server/main.go")
		return
	}

	providers, initialProvider, initialModel := createProviders()

	cfg := agent.Config{ServerPath: serverPath}
	if initialModel != "" {
		cfg.Model = initialModel
	}

	agentInstance := agent.NewAgent(cfg, initialProvider)

	if err := agentInstance.Connect(ctx); err != nil {
		log.Printf("Failed to connect to MCP server: %v", err)
		return
	}
	log.Printf("Connected to MCP server")
	defer agentInstance.Close()

	if err := agentInstance.GetTools(ctx); err != nil {
		log.Printf("Failed to get MCP tools: %v", err)
		return
	}

	srv, err := web.NewServer(ctx, agentInstance, providers)
	if err != nil {
		log.Printf("Failed to create web server: %v", err)
		return
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting Keylime MCP Agent at http://localhost%s", addr)

	if err := srv.Start(addr); err != nil {
		log.Printf("Server error: %v", err)
		return
	}
}

func createProviders() ([]agent.LLMProvider, agent.LLMProvider, string) {
	ollamaURL := os.Getenv("OLLAMA_URL")
	ollamaModel := os.Getenv("OLLAMA_MODEL")
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))

	if ollamaURL == "" {
		ollamaURL = defaultOllamaURL
	}

	var providers []agent.LLMProvider

	var claudeProvider *agent.AnthropicProvider
	if apiKey != "" {
		claudeProvider = agent.NewClaudeProvider(apiKey)
		providers = append(providers, claudeProvider)
	}

	ollamaProvider := agent.NewOllamaProvider(ollamaURL)
	providers = append(providers, ollamaProvider)

	if ollamaModel != "" || os.Getenv("OLLAMA_URL") != "" {
		log.Printf("Using Ollama provider at %s", ollamaURL)
		return providers, ollamaProvider, ollamaModel
	}

	if claudeProvider == nil {
		log.Fatal("Set ANTHROPIC_API_KEY for Claude or OLLAMA_URL/OLLAMA_MODEL for local Ollama")
	}

	log.Printf("Using Claude provider")
	return providers, claudeProvider, ""
}
