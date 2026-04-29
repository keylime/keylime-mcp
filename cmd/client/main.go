package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/keylime/keylime-mcp/internal/agent"
	"github.com/keylime/keylime-mcp/internal/masking"
	"github.com/keylime/keylime-mcp/internal/web"
)

type config struct {
	ServerPath     string
	Port           string
	OllamaURL      string
	OllamaModel    string
	AnthropicKey   string
	MaskingEnabled bool
}

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

	cfg := loadConfig()

	if _, err := os.Stat(cfg.ServerPath); os.IsNotExist(err) { // #nosec G703 -- serverPath from env/default, not user input
		log.Printf("Warning: MCP server not found at %s", cfg.ServerPath)
		log.Printf("Build the server first: go build -o bin/server cmd/server/main.go")
		return
	}

	providers, initialProvider, initialModel := createProviders(cfg)

	agentCfg := agent.Config{ServerPath: cfg.ServerPath, Model: initialModel}
	if agentCfg.Model == "" {
		if models, err := initialProvider.ListModels(ctx); err == nil && len(models) > 0 {
			agentCfg.Model = models[0].ID
			log.Printf("Auto-selected model: %s", agentCfg.Model)
		}
	}

	masker := masking.NewEngine(cfg.MaskingEnabled)
	agentInstance := agent.NewAgent(agentCfg, initialProvider, masker)

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

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting Keylime MCP Agent at http://localhost%s", addr)

	if err := srv.Start(addr); err != nil {
		log.Printf("Server error: %v", err)
		return
	}
}

func loadConfig() config {
	if err := godotenv.Load("./../.env"); err != nil {
		log.Printf("Warning: .env file not loaded: %v", err)
	}
	return config{
		ServerPath:     getEnv("MCP_SERVER_PATH", "./server"),
		Port:           getEnv("PORT", "3000"),
		OllamaURL:      getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:    os.Getenv("OLLAMA_MODEL"),
		AnthropicKey:   strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")),
		MaskingEnabled: parseBool(getEnv("MASKING_ENABLED", "true")),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseBool(s string) bool {
	v, _ := strconv.ParseBool(s)
	return v
}

func createProviders(cfg config) ([]agent.LLMProvider, agent.LLMProvider, string) {
	var providers []agent.LLMProvider

	var claudeProvider *agent.AnthropicProvider
	if cfg.AnthropicKey != "" {
		claudeProvider = agent.NewClaudeProvider(cfg.AnthropicKey)
		providers = append(providers, claudeProvider)
	}

	ollamaProvider := agent.NewOllamaProvider(cfg.OllamaURL)
	providers = append(providers, ollamaProvider)

	if cfg.OllamaModel != "" || os.Getenv("OLLAMA_URL") != "" {
		log.Printf("Using Ollama provider at %s", cfg.OllamaURL)
		return providers, ollamaProvider, cfg.OllamaModel
	}

	if claudeProvider == nil {
		log.Fatal("Set ANTHROPIC_API_KEY for Claude or OLLAMA_URL/OLLAMA_MODEL for local Ollama")
	}

	log.Printf("Using Claude provider")
	return providers, claudeProvider, ""
}
