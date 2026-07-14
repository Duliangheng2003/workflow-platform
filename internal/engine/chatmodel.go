package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/Duliangheng2003/workflow-platform/internal/config"
)

// chatModel implements eino's model.ToolCallingChatModel by wrapping our
// server-side LLM API client. This allows the eino react agent
// to use providers configured in config.yaml without needing eino-ext.
type chatModel struct {
	profile   *config.LLMProfile
	client    *http.Client
	toolInfos []*schema.ToolInfo
}

func newChatModel(profile *config.LLMProfile) *chatModel {
	return &chatModel{
		profile: profile,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// WithTools implements model.ToolCallingChatModel.
// Returns a new chatModel with the given tools attached, safe for concurrent use.
func (m *chatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	newModel := &chatModel{
		profile:   m.profile,
		client:    m.client,
		toolInfos: tools,
	}
	return newModel, nil
}

// Generate implements model.BaseChatModel.
// Uses tools from the store (set by WithTools) or from options.var chatModel
func (m *chatModel) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	// Collect options
	opt := model.GetCommonOptions(nil, opts...)

	// Convert eino messages to OpenAI API format
	apiMessages := make([]map[string]any, 0, len(msgs))
	for _, msg := range msgs {
		apiMsg := make(map[string]any)
		apiMsg["role"] = msg.Role

		// Handle tool calls in assistant messages
		if len(msg.ToolCalls) > 0 {
			tcList := make([]map[string]any, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				tcList = append(tcList, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			apiMsg["content"] = msg.Content
			apiMsg["tool_calls"] = tcList
		} else if msg.Role == "tool" {
			apiMsg["content"] = msg.Content
			apiMsg["tool_call_id"] = msg.ToolCallID
		} else {
			apiMsg["content"] = msg.Content
		}

		apiMessages = append(apiMessages, apiMsg)
	}

	// Build request body
	reqBody := map[string]any{
		"model":    m.profile.Model,
		"messages": apiMessages,
	}

	// Add tool definitions if provided
	tools := opt.Tools
	if len(tools) == 0 {
		tools = m.toolInfos
	}
	if len(tools) > 0 {
		toolsList := make([]map[string]any, 0, len(opt.Tools))
		for _, ti := range opt.Tools {
			toolsList = append(toolsList, toolInfoToOpenAI(ti))
		}
		reqBody["tools"] = toolsList
	}

	body, _ := json.Marshal(reqBody)

	// Build API URL
	baseURL := strings.TrimRight(m.profile.BaseURL, "/")
	apiURL := baseURL + "/v1/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.profile.APIKey)

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call LLM: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error debugging
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("LLM API %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if apiResp.Error != nil && apiResp.Error.Message != "" {
		return nil, fmt.Errorf("LLM error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	choice := apiResp.Choices[0]
	result := &schema.Message{
		Role:    schema.RoleType(choice.Message.Role),
		Content: choice.Message.Content,
	}

	// Convert tool calls from response
	if len(choice.Message.ToolCalls) > 0 {
		for _, tc := range choice.Message.ToolCalls {
			result.ToolCalls = append(result.ToolCalls, schema.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	return result, nil
}

// Stream implements model.BaseChatModel.
// Not supported for now — returns an error.
func (m *chatModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("streaming not supported for agent chat model")
}

// toolInfoToOpenAI converts eino's ToolInfo to OpenAI's tool definition format.
func toolInfoToOpenAI(ti *schema.ToolInfo) map[string]any {
	params := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	if ti.ParamsOneOf != nil {
		if js, err := ti.ParamsOneOf.ToJSONSchema(); err == nil && js != nil {
			if b, err := json.Marshal(js); err == nil {
				var jsMap map[string]any
				if json.Unmarshal(b, &jsMap) == nil {
					params = jsMap
				}
			}
		}
	}
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        ti.Name,
			"description": ti.Desc,
			"parameters":  params,
		},
	}
}