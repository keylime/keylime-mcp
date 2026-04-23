package masking

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func WrapTool[In, Out any](engine *Engine, handler mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	if !engine.Enabled() {
		return handler
	}

	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		var zero Out

		inputJSON, err := json.Marshal(input)
		if err != nil {
			return nil, zero, err
		}
		unmasked := engine.Unmask(string(inputJSON))
		if unmasked != string(inputJSON) {
			if err := json.Unmarshal([]byte(unmasked), &input); err != nil {
				return nil, zero, err
			}
		}

		result, output, err := handler(ctx, req, input)

		if err != nil {
			return nil, zero, errors.New(engine.Mask(err.Error()))
		}

		outputJSON, err := json.Marshal(output)
		if err != nil {
			return result, output, nil
		}
		masked := engine.Mask(string(outputJSON))
		if masked == "null" {
			return result, output, nil
		}
		if result == nil {
			result = &mcp.CallToolResult{}
		}
		result.Content = []mcp.Content{&mcp.TextContent{Text: masked}}
		result.StructuredContent = json.RawMessage(masked)
		return result, zero, nil
	}
}
