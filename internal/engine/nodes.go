package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/Duliangheng2003/workflow-platform/internal/model"
)

// callLambda creates a Lambda that calls a user-configured webhook.
func (e *Engine) callLambda(node *model.Node) func(context.Context, map[string]any) (map[string]any, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	return func(ctx context.Context, state map[string]any) (map[string]any, error) {
		method := node.Method
		if method == "" {
			method = http.MethodPost
		}

		var bodyReader *bytes.Reader
		contentType := "application/json"

		switch node.BodyType {
		case "none":
			bodyReader = bytes.NewReader(nil)
		case "raw":
			bodyStr := resolveTemplate(node.BodyContent, state)
			bodyReader = bytes.NewReader([]byte(bodyStr))
			contentType = "text/plain"
		case "json":
			bodyStr := resolveTemplate(node.BodyContent, state)
			if bodyStr != "" {
				bodyReader = bytes.NewReader([]byte(bodyStr))
			} else {
				stateJSON, _ := json.Marshal(state)
				bodyReader = bytes.NewReader(stateJSON)
			}
		default:
			body, err := json.Marshal(state)
			if err != nil {
				return nil, fmt.Errorf("marshal state: %w", err)
			}
			bodyReader = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, node.WebhookURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", contentType)

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("call webhook: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("webhook returned %d", resp.StatusCode)
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		state[node.ID] = result
		return state, nil
	}
}

// conditionLambda creates a Lambda that evaluates a condition expression
// and stores the result in state[node.ID].result.
func (e *Engine) conditionLambda(node *model.Node) func(context.Context, map[string]any) (map[string]any, error) {
	return func(ctx context.Context, state map[string]any) (map[string]any, error) {
		result, err := evaluateCondition(state, node)
		if err != nil {
			return nil, fmt.Errorf("evaluate condition: %w", err)
		}
		state[node.ID] = map[string]any{"result": result}
		return state, nil
	}
}

// humanLambda creates a Lambda that pauses execution for human input.
func (e *Engine) humanLambda(tmpl *model.Template, node *model.Node) func(context.Context, map[string]any) (map[string]any, error) {
	return func(ctx context.Context, state map[string]any) (map[string]any, error) {
		// Create a human task record
		task := &model.HumanTask{
			TemplateID:      tmpl.ID,
			NodeID:          node.ID,
			NodeDescription: node.Description,
			AssigneeGroup:   node.AssigneeGroup,
			Status:          model.HumanTaskPending,
			InputData:       copyMap(state),
		}

		// We need the instance ID from context (set during StartInstance)
		instID := getInstanceID(ctx)
		if instID != "" {
			task.InstanceID = instID
			// Update instance status to paused
			inst, err := e.store.GetInstance(instID)
			if err == nil {
				inst.Status = model.StatusPaused
				if inst.NodeStates[node.ID] != nil {
					inst.NodeStates[node.ID].Status = "paused"
				}
				_ = e.store.UpdateInstance(inst)
			}
		}

		_ = e.store.CreateHumanTask(task)

		// Block and wait for resume signal
		e.mu.RLock()
		ch, ok := e.waiters[instID]
		e.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("no resume channel for instance %s", instID)
		}

		select {
		case signal := <-ch:
			state[node.ID] = map[string]any{
				"approved": signal.Action == "approve",
				"result":   signal.Result,
			}
			return state, nil

		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for human input")
		}
	}
}

// ——————————————————————————————————————————————————————————————
// Condition evaluation
// ——————————————————————————————————————————————————————————————

func evaluateCondition(state map[string]any, node *model.Node) (bool, error) {
	expr := strings.TrimSpace(node.Expression)
	if expr == "" {
		return false, fmt.Errorf("empty expression")
	}

	var operator string
	var parts []string

	if strings.Contains(expr, "!=") {
		operator = "!="
		parts = strings.SplitN(expr, "!=", 2)
	} else if strings.Contains(expr, "=") {
		operator = "="
		parts = strings.SplitN(expr, "=", 2)
	} else {
		operator = "truthy"
		parts = []string{expr, ""}
	}

	path := strings.TrimSpace(parts[0])
	expected := ""
	if len(parts) > 1 {
		expected = strings.TrimSpace(parts[1])
		expected = strings.Trim(expected, "\"'")
	}

	segments := strings.Split(path, ".")
	if len(segments) < 2 || segments[0] != "state" {
		return false, fmt.Errorf("expression must start with 'state.'")
	}

	actual := resolvePath(state, segments[1:])

	switch operator {
	case "=":
		return fmt.Sprintf("%v", actual) == expected, nil
	case "!=":
		return fmt.Sprintf("%v", actual) != expected, nil
	default:
		if actual == nil {
			return false, nil
		}
		if s, ok := actual.(string); ok {
			return s != "", nil
		}
		if b, ok := actual.(bool); ok {
			return b, nil
		}
		return true, nil
	}
}

// ——————————————————————————————————————————————————————————————
// Utilities
// ——————————————————————————————————————————————————————————————

func copyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func resolvePath(state map[string]any, segments []string) any {
	current := any(state)
	for _, seg := range segments {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[seg]
		if current == nil {
			return nil
		}
	}
	return current
}

// codeLambda creates a Lambda that executes a JS/Python script.
func (e *Engine) codeLambda(node *model.Node) func(context.Context, map[string]any) (map[string]any, error) {
	return func(ctx context.Context, state map[string]any) (map[string]any, error) {
		script := node.Code
		lang := node.Language
		if lang == "" {
			lang = "js"
		}

		// Serialize input data as JSON
		inputJSON, _ := json.Marshal(state)
		input := string(inputJSON)

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
			state[node.ID] = map[string]any{"error": fmt.Sprintf("unsupported language: %s", lang)}
			return state, nil
		}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			state[node.ID] = map[string]any{
				"error":  fmt.Sprintf("script error: %v", err),
				"stderr": stderr.String(),
			}
			return state, nil
		}

		state[node.ID] = map[string]any{
			"language": lang,
			"output":   strings.TrimSpace(stdout.String()),
			"stderr":   strings.TrimSpace(stderr.String()),
		}
		return state, nil
	}
}

// extractorLambda creates a Lambda that uses an LLM to extract
// structured information from uploaded files and passes the result
// to the Agent via Data edge.
func (e *Engine) extractorLambda(node *model.Node) func(context.Context, map[string]any) (map[string]any, error) {
	return func(ctx context.Context, state map[string]any) (map[string]any, error) {
		if node.LLMProfile == "" {
			// No LLM configured, return raw file info
			state[node.ID] = map[string]any{
				"file_name": node.FileName,
				"summary":   fmt.Sprintf("File: %s (no LLM configured for extraction)", node.FileName),
			}
			return state, nil
		}

		profile, err := e.llmProfiles.LookupProfile(node.LLMProfile)
		if err != nil {
			return nil, fmt.Errorf("extractor %s: %w", node.ID, err)
		}

		// Build the extraction prompt
		extractPrompt := node.ExtractPrompt
		if extractPrompt == "" {
			extractPrompt = "Extract key information from this file and provide a concise summary."
		}

		userPrompt := fmt.Sprintf("File name: %s\n\nFile content:\n%s\n\nTask: %s",
			node.FileName, node.FileContent, extractPrompt)

		systemPrompt := "You are a data extraction assistant. Extract and organize the key information from the provided file content. Return a structured summary."

		content, err := callLLM(ctx, profile, systemPrompt, userPrompt, 0.3, 4096)
		if err != nil {
			// Fallback: return raw info if LLM fails
			state[node.ID] = map[string]any{
				"file_name": node.FileName,
				"summary":   fmt.Sprintf("File: %s (extraction failed: %s)", node.FileName, err.Error()),
			}
			return state, nil
		}

		state[node.ID] = map[string]any{
			"file_name": node.FileName,
			"summary":   content,
		}
		return state, nil
	}
}