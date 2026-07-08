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
// The agent runs a ReAct loop: it can think, call tools (code nodes),
// observe results, and decide what to do next.
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

		// 3. Build tools from referenced code nodes
		var tools []tool.BaseTool
		for _, tid := range cfg.Tools {
			n := findNode(tmpl.Nodes, tid)
			if n == nil {
				continue
			}
			switch n.Type {
			case model.NodeTypeCode:
				tools = append(tools, newCodeNodeTool(n, state))
			case model.NodeTypeHuman:
				tools = append(tools, newHumanTaskTool(n, state))
			}
		}

		// 4. Determine max iterations
		maxTurns := cfg.MaxTurns
		if maxTurns <= 0 {
			maxTurns = 10
		}

		// 5. Set up the agent input
		stateJSON, _ := json.MarshalIndent(state, "", "  ")
		inputMsg := fmt.Sprintf("Current workflow state:\n```json\n%s\n```\n\nAnalyze the situation and take appropriate actions. When you are done, provide a summary of what you did.", string(stateJSON))

		msgs := []*schema.Message{
			{Role: "system", Content: cfg.SystemPrompt},
			{Role: "user", Content: inputMsg},
		}

		// 6. Run the ReAct loop manually (avoids ADK Runner complexity)
		result, err := runReAct(ctx, cm, msgs, tools, maxTurns)
		if err != nil {
			return nil, fmt.Errorf("agent node %s: %w", node.ID, err)
		}

		// 7. Store result in state
		state[node.ID] = map[string]any{
			"content":   result.Content,
			"tool_calls": result.ToolCalls,
		}
		return state, nil
	}
}

// runReAct implements a simple ReAct loop: model → tool calls → model → ...
// Uses eino's schema.Message and tool.BaseTool for compatibility.
func runReAct(ctx context.Context, cm *chatModel, msgs []*schema.Message, tools []tool.BaseTool, maxTurns int) (*schema.Message, error) {
	// Build tool info list for the model
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
		// Call the model
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

		// Check if the model wants to call tools
		if len(resp.ToolCalls) == 0 {
			// No tool calls — agent is done
			return resp, nil
		}

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			t, ok := toolMap[tc.Function.Name]
			if !ok {
				// Tool not found — return error message
				msgs = append(msgs, &schema.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("Error: tool '%s' not found", tc.Function.Name),
				})
				continue
			}

			// Execute the tool
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

	// Max turns reached — return last response
	return &schema.Message{
		Role:    "assistant",
		Content: fmt.Sprintf("Reached maximum turns (%d). Last response: %s", maxTurns, lastContent),
	}, nil
}

// ——————————————————————————————————————————————————————————————
// Tool wrappers
// ——————————————————————————————————————————————————————————————

// codeNodeTool wraps a code node as an eino tool.BaseTool.
// When the model calls this tool, it executes the node's webhook.
type codeNodeTool struct {
	node  *model.Node
	state map[string]any
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
		desc = fmt.Sprintf("Call %s API", t.node.ID)
	}

	return &schema.ToolInfo{
		Name: t.node.ID,
		Desc: desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"input": {
				Type: schema.String,
				Desc: "Input data for the API call (JSON string)",
			},
		}),
	}, nil
}

func (t *codeNodeTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	// Merge tool input with current state
	bodyMap := make(map[string]any)
	bodyMap["tool_input"] = input

	// Try to parse the input as JSON and merge with state
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

	// Store result in state
	var result any
	if err := json.Unmarshal(respBody, &result); err == nil {
		t.state[t.node.ID] = result
	}

	return string(respBody), nil
}

// humanTaskTool wraps a human node as a tool — when the agent calls it,
// it creates a human task and pauses for input.
// For MVP, this creates a simple task that requires manual approval.
type humanTaskTool struct {
	node  *model.Node
	state map[string]any
}

func newHumanTaskTool(node *model.Node, state map[string]any) *humanTaskTool {
	return &humanTaskTool{
		node:  node,
		state: state,
	}
}

func (t *humanTaskTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	desc := t.node.Description
	if desc == "" {
		desc = "Request human input"
	}

	return &schema.ToolInfo{
		Name: t.node.ID,
		Desc: desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"request": {
				Type: schema.String,
				Desc: "What input is needed from the human",
			},
		}),
	}, nil
}

func (t *humanTaskTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	// For MVP, human tasks within agent tools return a placeholder
	// In production, this would create a real HumanTask and pause
	t.state[t.node.ID] = map[string]any{
		"agent_request": input,
		"status":        "pending_human_input",
	}

	return fmt.Sprintf("Human task created. Request: %s. Waiting for manual approval...", input), nil
}