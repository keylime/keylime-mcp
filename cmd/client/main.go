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

	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		log.Println("ANTHROPIC_API_KEY environment variable not set")
		return
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

	agentInstance := agent.NewAgent(agent.Config{
		APIKey:     apiKey,
		ServerPath: serverPath,
	})

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

	srv, err := web.NewServer(ctx, agentInstance)
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
