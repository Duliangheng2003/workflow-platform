package engine

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/compose"

	"github.com/Duliangheng2003/workflow-platform/internal/config"
	"github.com/Duliangheng2003/workflow-platform/internal/model"
	"github.com/Duliangheng2003/workflow-platform/internal/store"
)

// Engine is the workflow execution engine.
// It translates workflow templates into eino compose.Graph instances
// and manages their lifecycle.
type Engine struct {
	store       store.Store
	llmProfiles config.LLMConfig

	mu      sync.RWMutex
	waiters map[string]chan *resumeSignal // instanceID -> resume channel

	thinkingTraces sync.Map // instanceID -> *[]string (real-time Agent thinking trace)
}

type resumeSignal struct {
	Result interface{}
	Action string
}

func New(s store.Store, llmCfg config.LLMConfig) *Engine {
	return &Engine{
		store:       s,
		llmProfiles: llmCfg,
		waiters:     make(map[string]chan *resumeSignal),
	}
}

// StartCronScheduler starts a background goroutine that checks for
// Schedule-type templates and starts instances when cron matches.
func (e *Engine) StartCronScheduler(ctx context.Context) {
	go func() {
		log.Println("Cron scheduler started")
		for {
			select {
			case <-ctx.Done():
				log.Println("Cron scheduler stopped")
				return
			case <-time.After(1 * time.Minute):
				e.checkSchedules(ctx)
			}
		}
	}()
}

func (e *Engine) checkSchedules(ctx context.Context) {
	templates, err := e.store.ListTemplates()
	if err != nil {
		return
	}

	now := time.Now()
	for _, tmpl := range templates {
		if tmpl.StartType != "Schedule" || tmpl.CronExpr == "" {
			continue
		}
		if matchCron(tmpl.CronExpr, now) {
			// Start instance with empty input
			inst, err := e.StartInstance(ctx, tmpl.ID, nil)
			if err != nil {
				log.Printf("Cron: failed to start %s: %v", tmpl.ID, err)
			} else {
				log.Printf("Cron: started instance %s for template %s", inst.ID, tmpl.ID)
			}
		}
	}
}

// matchCron checks if a cron expression matches the given time.
// Format: minute hour day month weekday (0-59 0-23 1-31 1-12 0-6)
func matchCron(expr string, t time.Time) bool {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return false
	}

	return matchCronField(parts[0], t.Minute(), 0, 59) &&
		matchCronField(parts[1], t.Hour(), 0, 23) &&
		matchCronField(parts[2], t.Day(), 1, 31) &&
		matchCronField(parts[3], int(t.Month()), 1, 12) &&
		matchCronField(parts[4], int(t.Weekday()), 0, 6)
}

func matchCronField(field string, value int, min, max int) bool {
	if field == "*" {
		return true
	}
	// Handle "*/N" step syntax
	if strings.HasPrefix(field, "*/") {
		step := 0
		fmt.Sscanf(field, "*/%d", &step)
		if step > 0 && value%step == 0 {
			return true
		}
		return false
	}
	// Handle comma-separated values
	for _, part := range strings.Split(field, ",") {
		var v int
		if _, err := fmt.Sscanf(part, "%d", &v); err == nil && v == value {
			return true
		}
	}
	return false
}

// StartInstance creates a workflow instance and starts executing it.
func (e *Engine) StartInstance(ctx context.Context, tmplID string, input map[string]any) (*model.Instance, error) {
	tmpl, err := e.store.GetTemplate(tmplID)
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}

	state := make(map[string]any)
	state["_global"] = input

	nodeStates := make(map[string]*model.NodeExecutionState)
	for _, node := range tmpl.Nodes {
		nodeStates[node.ID] = &model.NodeExecutionState{NodeID: node.ID, Status: "pending"}
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

	// Update template's last run time
	now := time.Now()
	tmpl.LastRunAt = &now
	_ = e.store.UpdateTemplate(tmpl)

	// Build and compile the eino graph
	graph, err := e.buildGraph(tmpl)
	if err != nil {
		inst.Status = model.StatusFailed
		inst.Error = err.Error()
		_ = e.store.UpdateInstance(inst)
		return nil, fmt.Errorf("build graph: %w", err)
	}
	runnable, err := graph.Compile(ctx)
	if err != nil {
		inst.Status = model.StatusFailed
		inst.Error = err.Error()
		_ = e.store.UpdateInstance(inst)
		return nil, fmt.Errorf("compile graph: %w", err)
	}

	// Register resume channel for human tasks
	ch := make(chan *resumeSignal, 1)
	e.mu.Lock()
	e.waiters[inst.ID] = ch
	e.mu.Unlock()

	// Execute in background
	go func() {
		defer func() {
			e.mu.Lock()
			delete(e.waiters, inst.ID)
			e.mu.Unlock()
		}()

		// Embed instance info in context so lambdas can update state
		runCtx := withInstanceInfo(context.Background(), inst.ID)

		result, err := runnable.Invoke(runCtx, state)
		if err != nil {
			inst.Status = model.StatusFailed
			inst.Error = err.Error()
		} else if inst.Status != model.StatusPaused {
			inst.Status = model.StatusCompleted
			inst.State = result
			// Reload NodeStates to preserve updates from updateNodeState
			if latest, err := e.store.GetInstance(inst.ID); err == nil {
				inst.NodeStates = latest.NodeStates
			}
		}
		_ = e.store.UpdateInstance(inst)
	}()

	return inst, nil
}

// ResumeTask resumes a workflow instance paused at a human node.
func (e *Engine) ResumeTask(ctx context.Context, taskID, action string, result any) error {
	task, err := e.store.GetHumanTask(taskID)
	if err != nil {
		return fmt.Errorf("get human task: %w", err)
	}
	if task.Status != model.HumanTaskPending {
		return fmt.Errorf("task %s is not pending (status: %s)", taskID, task.Status)
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

// buildGraph translates a workflow template into an eino compose.Graph.
//
// Key mapping:
//   - code / llm / human nodes → Lambda nodes
//   - condition nodes → Branch on the predecessor node
//   - edges → AddEdge (except condition outputs, which use AddBranch)
//   - state flows as map[string]any through the graph
func (e *Engine) buildGraph(tmpl *model.Template) (*compose.Graph[map[string]any, map[string]any], error) {
	g := compose.NewGraph[map[string]any, map[string]any]()

	// Index condition nodes
	condNodes := make(map[string]bool)
	for _, n := range tmpl.Nodes {
		if n.Type == model.NodeTypeCondition {
			condNodes[n.ID] = true
		}
	}

	// Phase 1: Add all non-condition nodes as Lambdas
	for _, node := range tmpl.Nodes {
		if condNodes[node.ID] {
			continue
		}
		lambda, err := e.nodeToLambda(tmpl, &node)
		if err != nil {
			return nil, fmt.Errorf("create lambda %s: %w", node.ID, err)
		}
		if err := g.AddLambdaNode(node.ID, lambda); err != nil {
			return nil, fmt.Errorf("add lambda %s: %w", node.ID, err)
		}
	}

	// Phase 2: Add edges and branches
	for _, node := range tmpl.Nodes {
		if !condNodes[node.ID] {
			continue
		}

		// This is a condition node. Find its predecessor.
		pred := findPredecessor(tmpl.Edges, node.ID)

		// Create a Branch on the predecessor
		branchFunc := func(ctx context.Context, state map[string]any) (string, error) {
			e.updateNodeState(ctx, node.ID, "running", nil, "")
			result, err := evaluateCondition(state, &node)
			if v, ok := state["_global"].(map[string]any); ok { log.Printf("[Condition] _global.score=%v", v["score"]) }
			if err != nil {
				e.updateNodeState(ctx, node.ID, "failed", nil, err.Error())
				return "", err
			}
			port := "false"
			if result {
				port = "true"
			}
			state[node.ID] = map[string]any{"result": result, "port": port}
			e.updateNodeState(ctx, node.ID, "success", state[node.ID], "")
			log.Printf("[Condition %s] expression=%q result=%v selected_port=%s", node.ID, node.Expression, result, port)
			// Find the next node from the condition edge with matching output_port
			for _, e := range tmpl.Edges {
				if e.From == node.ID && e.OutputPort == port {
					return e.To, nil
				}
			}
			// Fallback: if no edge with exact port, pick the first flow edge from this node
			for _, e := range tmpl.Edges {
				if e.From == node.ID && e.EdgeType != model.EdgeTypeData {
					log.Printf("[Condition %s] using fallback edge to %s", node.ID, e.To)
					return e.To, nil
				}
			}
			return "", fmt.Errorf("no edge found for condition %s port %s", node.ID, port)
		}

		// Collect all possible next node IDs from condition edges as endNodes
		// eino requires all branch return values to be in endNodes
		endNodes := make(map[string]bool)
		for _, edge := range tmpl.Edges {
			if edge.From == node.ID {
				endNodes[edge.To] = true
			}
		}

		gb := compose.NewGraphBranch[map[string]any](branchFunc, endNodes)

		source := pred
		if source == "START" || source == "_start" {
			source = compose.START
		}
		if err := g.AddBranch(source, gb); err != nil {
			return nil, fmt.Errorf("add branch %s: %w", node.ID, err)
		}
	}

	// Phase 3: Add regular edges (skip condition nodes, Data edges, START/END edges)
	for _, edge := range tmpl.Edges {
		if condNodes[edge.From] || condNodes[edge.To] {
			continue
		}
		if edge.EdgeType == model.EdgeTypeData {
			continue
		}
		from := edge.From
		to := edge.To
		if from == "START" || from == "_start" {
			from = compose.START
		}
		if to == "END" {
			to = compose.END
		}
		if from != compose.START && to != compose.END {
			if err := g.AddEdge(from, to); err != nil {
				return nil, fmt.Errorf("add edge %s->%s: %w", edge.From, edge.To, err)
			}
		}
	}

	// Phase 4: Connect START to adjacent nodes
	for _, edge := range tmpl.Edges {
		if edge.EdgeType == model.EdgeTypeData {
			continue
		}
		if edge.From == "START" || edge.From == "_start" {
			if !condNodes[edge.To] {
				if err := g.AddEdge(compose.START, edge.To); err != nil {
					return nil, fmt.Errorf("add START->%s: %w", edge.To, err)
				}
			}
		}
	}

	// Phase 5: Auto-connect leaf nodes (nodes with no outgoing flow edges) to END
	hasOutgoing := make(map[string]bool)
	for _, edge := range tmpl.Edges {
		if edge.EdgeType == model.EdgeTypeData {
			continue
		}
		if edge.To == "END" {
			continue
		}
		hasOutgoing[edge.From] = true
	}
	for _, node := range tmpl.Nodes {
		if condNodes[node.ID] {
			continue
		}
		if !hasOutgoing[node.ID] {
			if err := g.AddEdge(node.ID, compose.END); err != nil {
				return nil, fmt.Errorf("add %s->END: %w", node.ID, err)
			}
		}
	}

	return g, nil
}
func (e *Engine) nodeToLambda(tmpl *model.Template, node *model.Node) (*compose.Lambda, error) {
	var inner func(context.Context, map[string]any) (map[string]any, error)
	switch node.Type {
	case model.NodeTypeCall:
		inner = e.callLambda(node)
	case model.NodeTypeLLM:
		if node.LLMConfig == nil {
			return nil, fmt.Errorf("llm node %s missing config", node.ID)
		}
		inner = e.llmLambda(node)
	case model.NodeTypeHuman:
		inner = e.humanLambda(tmpl, node)
	case model.NodeTypeAgent:
		if node.AgentConfig == nil {
			return nil, fmt.Errorf("agent node %s missing config", node.ID)
		}
		inner = e.agentLambda(tmpl, node)
	case model.NodeTypeCode:
		inner = e.codeLambda(node)
	case model.NodeTypeFilter:
		inner = e.codeLambda(node)
	case model.NodeTypeExtractor:
		inner = e.extractorLambda(node)
	default:
		return nil, fmt.Errorf("unsupported node type: %s", node.Type)
	}
	nodeID := node.ID
	return compose.InvokableLambda(func(ctx context.Context, state map[string]any) (map[string]any, error) {
		e.updateNodeState(ctx, nodeID, "running", nil, "")
		result, err := inner(ctx, state)
		if err != nil {
			e.updateNodeState(ctx, nodeID, "failed", nil, err.Error())
		} else {
			e.updateNodeState(ctx, nodeID, "success", result[nodeID], "")
		}
		return result, err
	}), nil
}

func findPredecessor(edges []model.Edge, nodeID string) string {
	for _, e := range edges {
		if e.To == nodeID && e.EdgeType != model.EdgeTypeData {
			return e.From
		}
	}
	return ""
}

func findNode(nodes []model.Node, id string) *model.Node {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

// AddThinkingStep appends a step to the real-time thinking trace for an instance.
func (e *Engine) AddThinkingStep(instID, step string) {
	if instID == "" {
		return
	}
	val, _ := e.thinkingTraces.LoadOrStore(instID, &[]string{})
	trace := val.(*[]string)
	*trace = append(*trace, step)
}

// GetThinkingTrace returns the current thinking trace for an instance.
func (e *Engine) GetThinkingTrace(instID string) []string {
	val, ok := e.thinkingTraces.Load(instID)
	if !ok {
		return nil
	}
	trace := val.(*[]string)
	return *trace
}

// ClearThinkingTrace removes the thinking trace for an instance.
func (e *Engine) ClearThinkingTrace(instID string) {
	e.thinkingTraces.Delete(instID)
}

// ——————————————————————————————————————————————————————————————
// Context helpers
// ——————————————————————————————————————————————————————————————

type instIDKey struct{}

func withInstanceInfo(ctx context.Context, instID string) context.Context {
	return context.WithValue(ctx, instIDKey{}, instID)
}

func getInstanceID(ctx context.Context) string {
	v, _ := ctx.Value(instIDKey{}).(string)
	return v
}

// updateNodeState updates the execution status of a node in the instance.
func (e *Engine) updateNodeState(ctx context.Context, nodeID string, status string, output interface{}, nodeErr string) {
	instID := getInstanceID(ctx)
	if instID == "" {
		return
	}
	inst, err := e.store.GetInstance(instID)
	if err != nil {
		return
	}
	if inst.NodeStates[nodeID] == nil {
		inst.NodeStates[nodeID] = &model.NodeExecutionState{NodeID: nodeID}
	}
	inst.NodeStates[nodeID].Status = status
	inst.NodeStates[nodeID].Output = output
	inst.NodeStates[nodeID].Error = nodeErr
	_ = e.store.UpdateInstance(inst)
}