package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Duliangheng2003/workflow-platform/internal/model"
)

// llmLambda creates a Lambda that calls an LLM via OpenAI-compatible API.
func (e *Engine) llmLambda(node *model.Node) func(context.Context, map[string]any) (map[string]any, error) {
	cfg := node.LLMConfig
	client := &http.Client{Timeout: 60 * time.Second}

	return func(ctx context.Context, state map[string]any) (map[string]any, error) {
		// Read API key from environment
		apiKey := os.Getenv(cfg.APIKeyEnv)
		if apiKey == "" {
			return nil, fmt.Errorf("API key not found in env: %s", cfg.APIKeyEnv)
		}

		// Resolve prompt template with state values
		userPrompt := resolveTemplate(cfg.UserPrompt, state)
		systemPrompt := cfg.SystemPrompt
		if systemPrompt != "" {
			systemPrompt = resolveTemplate(systemPrompt, state)
		}

		// Build the request body (OpenAI-compatible format)
		messages := make([]map[string]any, 0)
		if systemPrompt != "" {
			messages = append(messages, map[string]any{
				"role":    "system",
				"content": systemPrompt,
			})
		}
		messages = append(messages, map[string]any{
			"role":    "user",
			"content": userPrompt,
		})

		reqBody := map[string]any{
			"model":    cfg.ModelName,
			"messages": messages,
		}
		if cfg.Temperature != 0 {
			reqBody["temperature"] = cfg.Temperature
		}
		if cfg.MaxTokens != 0 {
			reqBody["max_tokens"] = cfg.MaxTokens
		}

		body, _ := json.Marshal(reqBody)

		// Determine API URL
		baseURL := cfg.BaseURL
		if baseURL == "" {
			switch cfg.Provider {
			case "deepseek":
				baseURL = "https://api.deepseek.com"
			default: // openai
				baseURL = "https://api.openai.com"
			}
		}
		apiURL := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("call LLM API: %w", err)
		}
		defer resp.Body.Close()

		var apiResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		if apiResp.Error != nil && apiResp.Error.Message != "" {
			return nil, fmt.Errorf("LLM API error: %s", apiResp.Error.Message)
		}

		if len(apiResp.Choices) == 0 {
			return nil, fmt.Errorf("LLM returned no choices")
		}

		content := apiResp.Choices[0].Message.Content
		state[node.ID] = map[string]any{
			"content": content,
		}
		return state, nil
	}
}

// resolveTemplate replaces {state.path.to.value} with actual state values.
func resolveTemplate(tmpl string, state map[string]any) string {
	re := regexp.MustCompile(`\{state\.([^}]+)\}`)
	return re.ReplaceAllStringFunc(tmpl, func(match string) string {
		path := strings.TrimPrefix(match, "{state.")
		path = strings.TrimSuffix(path, "}")
		segments := strings.Split(path, ".")
		val := resolvePath(state, segments)
		if val == nil {
			return match
		}
		b, _ := json.Marshal(val)
		s := string(b)
		// If it's a plain string, remove the JSON quotes
		if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
			return s[1 : len(s)-1]
		}
		return s
	})
}