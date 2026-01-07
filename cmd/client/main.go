package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/keylime/keylime-mcp/internal/agent"
)

const (
	defaultServerPath = "../../cmd/server/server"
	defaultPort       = "3000"
	envFileLocation   = "../../.env"
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

	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable not set")
	}

	serverPath := os.Getenv("MCP_SERVER_PATH")
	if serverPath == "" {
		serverPath = defaultServerPath
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		log.Printf("Warning: MCP server not found at %s", serverPath)
		log.Printf("Build the server first: go build -o bin/server cmd/server/main.go")
		return
	}

	agent := agent.NewAgent(agent.Config{
		APIKey:     apiKey,
		ServerPath: serverPath,
	})

	if err := agent.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to MCP server: %v", err)
	}
	log.Printf("Connected to MCP server")
	defer agent.Close()

	if err := agent.GetTools(ctx); err != nil {
		log.Fatalf("Failed to get MCP tools: %v", err)
	}
}
