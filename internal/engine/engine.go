package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Duliangheng2003/workflow-platform/internal/model"
	"github.com/Duliangheng2003/workflow-platform/internal/store"
)

// Engine is the core workflow execution engine.
// It translates workflow templates into executable DAGs and manages
// their lifecycle, including human task pause/resume.
type Engine struct {
	store store.Store

	mu         sync.RWMutex
	waiters    map[string]chan *resumeSignal // instanceID -> resume channel
	httpClient *http.Client
}

type resumeSignal struct {
	Result interface{}
	Action string // "approve" or "reject"
}

func New(s store.Store) *Engine {
	return &Engine{
		store:      s,
		waiters:    make(map[string]chan *resumeSignal),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// StartInstance creates a new workflow instance and begins executing it.
func (e *Engine) StartInstance(ctx context.Context, tmplID string, input map[string]interface{}) (*model.Instance, error) {
	tmpl, err := e.store.GetTemplate(tmplID)
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}

	// Validate template
	if err := validateTemplate(tmpl); err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}

	state := make(map[string]interface{})
	state["_global"] = input

	nodeStates := make(map[string]*model.NodeExecutionState)
	for _, node := range tmpl.Nodes {
		nodeStates[node.ID] = &model.NodeExecutionState{
			NodeID: node.ID,
			Status: "pending",
		}
	}

	inst := &model.Instance{
		TemplateID: tmplID,
		Status:     model.StatusRunning,
		State:      state,
		NodeStates: nodeStates,
	}

	if err := e.store.CreateInstance(inst); err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}

	// Execute in background
	go e.execute(context.Background(), inst, tmpl)

	return inst, nil
}

// ResumeTask resumes a paused workflow instance when a human task is completed.
func (e *Engine) ResumeTask(ctx context.Context, taskID, action string, result interface{}) error {
	task, err := e.store.GetHumanTask(taskID)
	if err != nil {
		return fmt.Errorf("get human task: %w", err)
	}
	if task.Status != model.HumanTaskPending {
		return fmt.Errorf("task %s is not pending, current status: %s", taskID, task.Status)
	}

	task.Status = model.HumanTaskApproved
	if action == "reject" {
		task.Status = model.HumanTaskRejected
	}
	task.Result = result
	_ = e.store.UpdateHumanTask(task)

	e.mu.RLock()
	ch, ok := e.waiters[task.InstanceID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no running instance found for %s", task.InstanceID)
	}

	ch <- &resumeSignal{Action: action, Result: result}
	return nil
}

// ——————————————————————————————————————————————————————————————
// Internal execution
// ——————————————————————————————————————————————————————————————

func (e *Engine) execute(ctx context.Context, inst *model.Instance, tmpl *model.Template) {
	// Register resume channel
	ch := make(chan *resumeSignal, 1)
	e.mu.Lock()
	e.waiters[inst.ID] = ch
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.waiters, inst.ID)
		e.mu.Unlock()
	}()

	// Build adjacency from edges (ignoring output_port)
	adj := buildAdjacency(tmpl.Edges)

	// Find start nodes
	queue := findStartNodes(tmpl.Edges)
	visited := make(map[string]bool)

	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]

		if visited[nodeID] {
			continue
		}
		visited[nodeID] = true

		node := findNode(tmpl.Nodes, nodeID)
		if node == nil {
			e.failInstance(inst, fmt.Sprintf("node not found: %s", nodeID))
			return
		}

		inst.CurrentNodeID = nodeID
		inst.NodeStates[nodeID].Status = "running"
		_ = e.store.UpdateInstance(inst)

		switch node.Type {
		case model.NodeTypeCode:
			if err := e.executeCodeNode(ctx, inst, node); err != nil {
				e.failInstance(inst, fmt.Sprintf("code node %s: %v", nodeID, err))
				return
			}
			inst.NodeStates[nodeID].Status = "completed"
			_ = e.store.UpdateInstance(inst)

			// Enqueue immediate successors
			queue = append(queue, adj[nodeID]...)

		case model.NodeTypeCondition:
			result, err := evaluateCondition(inst.State, node)
			if err != nil {
				e.failInstance(inst, fmt.Sprintf("condition node %s: %v", nodeID, err))
				return
			}
			inst.State[nodeID] = map[string]interface{}{"result": result}
			inst.NodeStates[nodeID].Status = "completed"
			inst.NodeStates[nodeID].Output = result
			_ = e.store.UpdateInstance(inst)

			// Enqueue only the matching branch
			next := findNextAfterCondition(tmpl.Edges, nodeID, result)
			queue = append(queue, next...)

		case model.NodeTypeHuman:
			task := &model.HumanTask{
				InstanceID:      inst.ID,
				TemplateID:      tmpl.ID,
				NodeID:          nodeID,
				NodeDescription: node.Description,
				AssigneeGroup:   node.AssigneeGroup,
				Status:          model.HumanTaskPending,
				InputData:       copyMap(inst.State),
			}
			_ = e.store.CreateHumanTask(task)

			inst.Status = model.StatusPaused
			inst.NodeStates[nodeID].Status = "paused"
			_ = e.store.UpdateInstance(inst)

			// Wait for human input
			select {
			case signal := <-ch:
				inst.State[nodeID] = map[string]interface{}{
					"approved": signal.Action == "approve",
					"result":   signal.Result,
				}
				inst.NodeStates[nodeID].Status = "completed"
				inst.NodeStates[nodeID].Output = signal.Result
				inst.Status = model.StatusRunning
				_ = e.store.UpdateInstance(inst)

				queue = append(queue, adj[nodeID]...)

			case <-ctx.Done():
				e.failInstance(inst, "context cancelled")
				return
			}
		}
	}

	inst.Status = model.StatusCompleted
	_ = e.store.UpdateInstance(inst)
}

func (e *Engine) executeCodeNode(ctx context.Context, inst *model.Instance, node *model.Node) error {
	body, err := json.Marshal(inst.State)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, node.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Instance-ID", inst.ID)
	req.Header.Set("X-Node-ID", node.ID)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode webhook response: %w", err)
	}

	inst.State[node.ID] = result
	inst.NodeStates[node.ID].Output = result
	return nil
}

func (e *Engine) failInstance(inst *model.Instance, errMsg string) {
	inst.Status = model.StatusFailed
	inst.Error = errMsg
	if inst.NodeStates[inst.CurrentNodeID] != nil {
		inst.NodeStates[inst.CurrentNodeID].Status = "failed"
		inst.NodeStates[inst.CurrentNodeID].Error = errMsg
	}
	_ = e.store.UpdateInstance(inst)
}

// ——————————————————————————————————————————————————————————————
// Condition evaluation
// ——————————————————————————————————————————————————————————————

func evaluateCondition(state map[string]interface{}, node *model.Node) (bool, error) {
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
		// truthy check
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

func resolvePath(state map[string]interface{}, segments []string) interface{} {
	current := interface{}(state)
	for _, seg := range segments {
		m, ok := current.(map[string]interface{})
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

// ——————————————————————————————————————————————————————————————
// Graph helpers
// ——————————————————————————————————————————————————————————————

func buildAdjacency(edges []model.Edge) map[string][]string {
	m := make(map[string][]string)
	for _, e := range edges {
		m[e.From] = append(m[e.From], e.To)
	}
	return m
}

func findStartNodes(edges []model.Edge) []string {
	var starts []string
	for _, e := range edges {
		if e.From == "START" {
			starts = append(starts, e.To)
		}
	}
	return starts
}

func findNode(nodes []model.Node, id string) *model.Node {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

func findNextAfterCondition(edges []model.Edge, from string, result bool) []string {
	branch := "false"
	if result {
		branch = "true"
	}
	for _, e := range edges {
		if e.From == from {
			if e.OutputPort == "" || e.OutputPort == branch {
				return []string{e.To}
			}
		}
	}
	return nil
}

func validateTemplate(tmpl *model.Template) error {
	if len(tmpl.Nodes) == 0 {
		return fmt.Errorf("no nodes defined")
	}
	if len(tmpl.Edges) == 0 {
		return fmt.Errorf("no edges defined")
	}
	// Check all referenced nodes exist
	ids := make(map[string]bool)
	for _, n := range tmpl.Nodes {
		ids[n.ID] = true
	}
	for _, e := range tmpl.Edges {
		if e.From != "START" && e.From != "END" && !ids[e.From] {
			return fmt.Errorf("edge references unknown node '%s'", e.From)
		}
		if e.To != "START" && e.To != "END" && !ids[e.To] {
			return fmt.Errorf("edge references unknown node '%s'", e.To)
		}
	}
	return nil
}

func copyMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}