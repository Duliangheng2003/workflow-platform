package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/flow/agent/react"
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
				tools = append(tools, newCodeScriptTool(n, state))
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
		// 7. Run the ReAct loop using eino's built-in react agent
		reactAgent, err := react.NewAgent(ctx, &react.AgentConfig{
			ToolCallingModel: cm,
			MaxStep:          maxTurns + 1,
		})
		if err != nil {
			return nil, fmt.Errorf("agent node %s: create react agent: %w", node.ID, err)
		}

		toolOpts, err := react.WithTools(ctx, tools...)
		if err != nil {
			return nil, fmt.Errorf("agent node %s: configure tools: %w", node.ID, err)
		}

		result, err := reactAgent.Generate(ctx, msgs, toolOpts...)
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
// codeScriptTool wraps a code node as an eino tool — when the Agent calls it,
// it executes the JS/Python script with the Agent's input and returns the output.
type codeScriptTool struct {
	node  *model.Node
	state map[string]any
}

func newCodeScriptTool(node *model.Node, state map[string]any) *codeScriptTool {
	return &codeScriptTool{
		node:  node,
		state: state,
	}
}

func (t *codeScriptTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	desc := t.node.Description
	if desc == "" {
		desc = fmt.Sprintf("Execute %s script", t.node.ID)
	}
	return &schema.ToolInfo{
		Name: t.node.ID,
		Desc: desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"input": {
				Type: schema.String,
				Desc: "Input data for the script (JSON string)",
			},
		}),
	}, nil
}

func (t *codeScriptTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	script := t.node.Code
	lang := t.node.Language
	if lang == "" {
		lang = "js"
	}

	var cmd *exec.Cmd
	switch lang {
	case "js", "javascript":
		wrapped := fmt.Sprintf("const input = %s; %s", input, script)
		cmd = exec.CommandContext(ctx, "node", "-e", wrapped)
	case "python", "py":
		escaped := strings.ReplaceAll(input, "'", "'\\''")
		code := fmt.Sprintf("import json; input = json.loads('%s'); %s", escaped, script)
		cmd = exec.CommandContext(ctx, "python3", "-c", code)
	default:
		return "", fmt.Errorf("unsupported language: %s", lang)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("script error: %v\nstderr: %s", err, stderr.String())
	}

	result := strings.TrimSpace(stdout.String())
	if stderr.Len() > 0 {
		result += "\n// stderr: " + strings.TrimSpace(stderr.String())
	}
	return result, nil
}
