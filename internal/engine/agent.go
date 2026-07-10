package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/Duliangheng2003/workflow-platform/internal/model"
)

// agentLambda creates a Lambda that executes an eino ChatModelAgent.
// The agent runs a ReAct loop: it can think, call tools, observe results,
// and decide what to do next. Data edges provide business context.
func (e *Engine) agentLambda(tmpl *model.Template, node *model.Node) func(context.Context, map[string]any) (map[string]any, error) {
	return func(ctx context.Context, state map[string]any) (map[string]any, error) {
		cfg := node.AgentConfig
		if cfg == nil {
			return nil, fmt.Errorf("agent node %s: missing config", node.ID)
		}

		// 1. Look up LLM profile
		profile, err := e.llmProfiles.LookupProfile(cfg.Profile)
		if err != nil {
			return nil, fmt.Errorf("agent node %s: %w", node.ID, err)
		}

		// 2. Create ChatModel wrapper
		cm := newChatModel(profile)

		// 3. Build tools from agent_config.tools (eino internal tool calling)
		var tools []tool.BaseTool
		for _, tid := range cfg.Tools {
			n := findNode(tmpl.Nodes, tid)
			if n == nil {
				continue
			}
			switch n.Type {
			case model.NodeTypeCall:
				tools = append(tools, newCodeNodeTool(n, state))
			case model.NodeTypeCode:
				tools = append(tools, newCodeNodeTool(n, state))
			}
		}

		// 4. Collect business context from Data edges
		// Data edges connect extractor/code nodes to provide business info
		var businessContext []string
		for _, edge := range tmpl.Edges {
			if edge.EdgeType != model.EdgeTypeData {
				continue
			}
			// Determine which side provides the context data
			contextID := edge.From
			if edge.From == node.ID {
				contextID = edge.To
			}
			n := findNode(tmpl.Nodes, contextID)
			if n == nil {
				continue
			}
			// Read the node's output from state
			if data, ok := state[contextID]; ok {
				dataJSON, _ := json.MarshalIndent(data, "", "  ")
				businessContext = append(businessContext, fmt.Sprintf(
					"## Data from %s (%s)\n```json\n%s\n```",
					n.ID, n.Type, string(dataJSON),
				))
			}
		}

		// 5. Determine max iterations
		maxTurns := cfg.MaxTurns
		if maxTurns <= 0 {
			maxTurns = 10
		}

		// 6. Set up the agent input with business context
		stateJSON, _ := json.MarshalIndent(state, "", "  ")
		inputMsg := fmt.Sprintf("Current workflow state:\n```json\n%s\n```", string(stateJSON))
		if len(businessContext) > 0 {
			inputMsg += "\n\nBusiness context:\n" + strings.Join(businessContext, "\n\n")
		}
		inputMsg += "\n\nAnalyze the situation and take appropriate actions. When you are done, provide a summary of what you did."

		msgs := []*schema.Message{
			{Role: "system", Content: cfg.SystemPrompt},
			{Role: "user", Content: inputMsg},
		}

		// 7. Run the ReAct loop
		result, err := runReAct(ctx, cm, msgs, tools, maxTurns)
		if err != nil {
			return nil, fmt.Errorf("agent node %s: %w", node.ID, err)
		}

		// 8. Store result in state
		state[node.ID] = map[string]any{
			"content":    result.Content,
			"tool_calls": result.ToolCalls,
		}
		return state, nil
	}
}

// runReAct implements a simple ReAct loop: model → tool calls → model → ...
func runReAct(ctx context.Context, cm *chatModel, msgs []*schema.Message, tools []tool.BaseTool, maxTurns int) (*schema.Message, error) {
	toolInfos := make([]*schema.ToolInfo, 0, len(tools))
	toolMap := make(map[string]tool.BaseTool)
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("tool info: %w", err)
		}
		toolInfos = append(toolInfos, info)
		toolMap[info.Name] = t
	}

	var lastContent string

	for turn := 0; turn < maxTurns; turn++ {
		opts := []einomodel.Option{}
		if len(toolInfos) > 0 {
			opts = append(opts, einomodel.WithTools(toolInfos))
		}

		resp, err := cm.Generate(ctx, msgs, opts...)
		if err != nil {
			return nil, fmt.Errorf("model call (turn %d): %w", turn, err)
		}

		msgs = append(msgs, resp)
		lastContent = resp.Content

		if len(resp.ToolCalls) == 0 {
			return resp, nil
		}

		for _, tc := range resp.ToolCalls {
			t, ok := toolMap[tc.Function.Name]
			if !ok {
				msgs = append(msgs, &schema.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("Error: tool '%s' not found", tc.Function.Name),
				})
				continue
			}

			var toolResult string
			invokable, ok := t.(tool.InvokableTool)
			if !ok {
				toolResult = fmt.Sprintf("Error: tool '%s' does not support execution", tc.Function.Name)
			} else {
				r, e := invokable.InvokableRun(ctx, tc.Function.Arguments)
				if e != nil {
					toolResult = fmt.Sprintf("Error: %v", e)
				} else {
					toolResult = r
				}
			}

			msgs = append(msgs, &schema.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    toolResult,
			})
		}
	}

	return &schema.Message{
		Role:    "assistant",
		Content: fmt.Sprintf("Reached maximum turns (%d). Last response: %s", maxTurns, lastContent),
	}, nil
}

// ——————————————————————————————————————————————————————————————
// Tool wrappers
// ——————————————————————————————————————————————————————————————

// codeNodeTool wraps a call/code node as an eino tool.BaseTool.
type codeNodeTool struct {
	node   *model.Node
	state  map[string]any
	client *http.Client
}

func newCodeNodeTool(node *model.Node, state map[string]any) *codeNodeTool {
	return &codeNodeTool{
		node:   node,
		state:  state,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *codeNodeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	desc := t.node.Description
	if desc == "" {
		desc = fmt.Sprintf("Call %s", t.node.ID)
	}
	return &schema.ToolInfo{
		Name: t.node.ID,
		Desc: desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"input": {
				Type: schema.String,
				Desc: "Input data (JSON string)",
			},
		}),
	}, nil
}

func (t *codeNodeTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	bodyMap := make(map[string]any)
	bodyMap["tool_input"] = input

	var inputJSON map[string]any
	if err := json.Unmarshal([]byte(input), &inputJSON); err == nil {
		for k, v := range inputJSON {
			bodyMap[k] = v
		}
	}
	bodyMap["_state"] = t.state
	body, _ := json.Marshal(bodyMap)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.node.WebhookURL, strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call webhook: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("webhook returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err == nil {
		t.state[t.node.ID] = result
	}
	return string(respBody), nil
}