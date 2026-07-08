package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/compose"

	"github.com/Duliangheng2003/workflow-platform/internal/model"
	"github.com/Duliangheng2003/workflow-platform/internal/store"
)

// Engine is the workflow execution engine.
// It translates workflow templates into eino compose.Graph instances
// and manages their lifecycle.
type Engine struct {
	store store.Store

	mu      sync.RWMutex
	waiters map[string]chan *resumeSignal // instanceID -> resume channel
}

type resumeSignal struct {
	Result interface{}
	Action string
}

func New(s store.Store) *Engine {
	return &Engine{
		store:   s,
		waiters: make(map[string]chan *resumeSignal),
	}
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

	// Build and compile the eino graph
	graph, err := e.buildGraph(tmpl)
	if err != nil {
		return nil, fmt.Errorf("build graph: %w", err)
	}
	runnable, err := graph.Compile(ctx)
	if err != nil {
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
			result, err := evaluateCondition(state, &node)
			if err != nil {
				return "", err
			}
			if result {
				return "true", nil
			}
			return "false", nil
		}

		// Determine end nodes (which branch outputs lead to END)
		endNodes := make(map[string]bool)
		for _, edge := range tmpl.Edges {
			if edge.From == node.ID && edge.To == "END" {
				endNodes[edge.OutputPort] = true
			}
		}

		gb := compose.NewGraphBranch[map[string]any](branchFunc, endNodes)

		source := pred
		if source == "START" {
			source = compose.START
		}
		if err := g.AddBranch(source, gb); err != nil {
			return nil, fmt.Errorf("add branch %s: %w", node.ID, err)
		}
	}

	// Phase 3: Add regular edges (skip condition nodes and their edges)
	for _, edge := range tmpl.Edges {
		// Skip edges involving condition nodes (they're handled by branches)
		if condNodes[edge.From] || condNodes[edge.To] {
			continue
		}
		from := edge.From
		to := edge.To
		if from == "START" {
			from = compose.START
		}
		if to == "END" {
			to = compose.END
		}
		if edge.From != "START" && edge.From != "END" && edge.To != "START" && edge.To != "END" {
			if err := g.AddEdge(from, to); err != nil {
				return nil, fmt.Errorf("add edge %s→%s: %w", edge.From, edge.To, err)
			}
		}
	}

	// Phase 4: Connect START/END to adjacent nodes
	for _, edge := range tmpl.Edges {
		if edge.From == "START" {
			if !condNodes[edge.To] {
				if err := g.AddEdge(compose.START, edge.To); err != nil {
					return nil, fmt.Errorf("add START→%s: %w", edge.To, err)
				}
			}
		}
		if edge.To == "END" {
			if !condNodes[edge.From] {
				if err := g.AddEdge(edge.From, compose.END); err != nil {
					return nil, fmt.Errorf("add %s→END: %w", edge.From, err)
				}
			}
		}
	}

	return g, nil
}

// nodeToLambda creates the correct Lambda wrapper for a node.
func (e *Engine) nodeToLambda(tmpl *model.Template, node *model.Node) (*compose.Lambda, error) {
	switch node.Type {
	case model.NodeTypeCode:
		return compose.InvokableLambda(e.codeLambda(node)), nil
	case model.NodeTypeLLM:
		if node.LLMConfig == nil {
			return nil, fmt.Errorf("llm node %s missing config", node.ID)
		}
		return compose.InvokableLambda(e.llmLambda(node)), nil
	case model.NodeTypeHuman:
		return compose.InvokableLambda(e.humanLambda(tmpl, node)), nil
	default:
		return nil, fmt.Errorf("unsupported node type: %s", node.Type)
	}
}

func findPredecessor(edges []model.Edge, nodeID string) string {
	for _, e := range edges {
		if e.To == nodeID {
			return e.From
		}
	}
	return ""
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