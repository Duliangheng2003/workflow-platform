package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Duliangheng2003/workflow-platform/internal/config"
	"github.com/Duliangheng2003/workflow-platform/internal/model"
)

// llmLambda creates a Lambda that calls an LLM via OpenAI-compatible API.
func (e *Engine) llmLambda(node *model.Node) func(context.Context, map[string]any) (map[string]any, error) {
	cfg := node.LLMConfig

	return func(ctx context.Context, state map[string]any) (map[string]any, error) {
		profile, err := e.llmProfiles.LookupProfile(cfg.Profile)
		if err != nil {
			return nil, fmt.Errorf("llm config: %w", err)
		}

		userPrompt := resolveTemplate(cfg.UserPrompt, state)
		systemPrompt := cfg.SystemPrompt
		if systemPrompt != "" {
			systemPrompt = resolveTemplate(systemPrompt, state)
		}

		content, err := callLLM(ctx, profile, systemPrompt, userPrompt, cfg.Temperature, cfg.MaxTokens)
		if err != nil {
			return nil, err
		}

		state[node.ID] = map[string]any{"content": content}
		return state, nil
	}
}

// callLLM calls an OpenAI-compatible LLM API and returns the response content.
func callLLM(ctx context.Context, profile *config.LLMProfile, systemPrompt, userPrompt string, temperature float64, maxTokens int) (string, error) {
	client := &http.Client{Timeout: 60 * time.Second}

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
		"model":    profile.Model,
		"messages": messages,
	}
	if temperature != 0 {
		reqBody["temperature"] = temperature
	}
	if maxTokens != 0 {
		reqBody["max_tokens"] = maxTokens
	}

	body, _ := json.Marshal(reqBody)

	baseURL := strings.TrimRight(profile.BaseURL, "/")
	apiURL := baseURL + "/v1/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+profile.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call LLM API: %w", err)
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
		return "", fmt.Errorf("decode response: %w", err)
	}
	if apiResp.Error != nil && apiResp.Error.Message != "" {
		return "", fmt.Errorf("LLM API error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	return apiResp.Choices[0].Message.Content, nil
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
		if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
			return s[1 : len(s)-1]
		}
		return s
	})
}