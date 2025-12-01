package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	webMode := flag.Bool("web", false, "Start web server mode")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupSignalHandler(cancel)

	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: .env file not loaded: %v", err)
	}

	config, err := NewConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	agent := NewAgent(config)
	if err := agent.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}
	defer agent.Close()

	if *webMode {
		runWebMode(ctx, agent)
	} else {
		runCLIMode(ctx, agent)
	}
}

func setupSignalHandler(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\nShutting down gracefully...")
		cancel()
	}()
}

func runWebMode(ctx context.Context, agent *Agent) {
	webServer, err := NewWebServer(agent)
	if err != nil {
		log.Fatalf("Failed to create web server: %v", err)
	}

	if err := webServer.Start(ctx, ":8080"); err != nil {
		log.Fatalf("Web server failed: %v", err)
	}
}

func runCLIMode(ctx context.Context, agent *Agent) {
	// TODO: Implement bubbletea terminal UI
	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("Usage: mcp-client [--web] <query>\nWeb mode: mcp-client --web\nCLI mode: mcp-client <your query>")
	}

	userQuery := strings.Join(args, " ")
	userQuery = strings.TrimSpace(userQuery)
	if userQuery == "" {
		log.Fatal("Error: user query cannot be empty")
	}

	if err := agent.ProcessQuery(ctx, userQuery); err != nil {
		log.Fatalf("Agent query failed: %v", err)
	}
}
