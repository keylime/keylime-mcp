package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type OllamaProvider struct {
	anthropicBase
	baseURL string
}

func NewOllamaProvider(baseURL string) *OllamaProvider {
	return &OllamaProvider{
		anthropicBase: anthropicBase{
			client: anthropic.NewClient(
				option.WithBaseURL(baseURL),
				option.WithAPIKey("ollama"),
			),
		},
		baseURL: baseURL,
	}
}

func (p *OllamaProvider) Name() string { return "ollama" }

func (p *OllamaProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpClient := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Ollama at %s: %w", p.baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Ollama response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &tagsResp); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama response: %w", err)
	}

	models := make([]ModelInfo, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		models = append(models, ModelInfo{
			ID:          m.Name,
			DisplayName: m.Name,
			Provider:    "ollama",
		})
	}

	return models, nil
}
