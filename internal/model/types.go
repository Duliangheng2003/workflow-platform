package model

import "time"

// NodeType defines the type of a workflow node.
type NodeType string

const (
	NodeTypeCode      NodeType = "code"
	NodeTypeCondition NodeType = "condition"
	NodeTypeHuman     NodeType = "human"
	NodeTypeLLM       NodeType = "llm"
	NodeTypeAgent     NodeType = "agent"
)

// LLMConfig defines configuration for an LLM node.
// API keys and model details are configured server-side in config.yaml
// under `llm.profiles` — the workflow template only references a profile name.
type LLMConfig struct {
	Profile      string  `json:"profile"`       // references a server-side LLM profile (e.g. "openai-gpt4o")
	SystemPrompt string  `json:"system_prompt"` // system prompt
	UserPrompt   string  `json:"user_prompt"`   // template with {state.path.to.value} syntax
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
}

// AgentConfig defines configuration for an agent node.
// The agent uses eino's ChatModelAgent with a ReAct loop, and can
// call other nodes in the workflow as tools.
type AgentConfig struct {
	Profile      string   `json:"profile"`       // LLM profile name from config.yaml
	SystemPrompt string   `json:"system_prompt"` // agent role/instruction
	Tools        []string `json:"tools"`         // node IDs that this agent can call as tools
	MaxTurns     int      `json:"max_turns"`     // max ReAct iterations (default 10)
}

// Node defines a single node in a workflow template.
type Node struct {
	ID            string    `json:"id"`
	Type          NodeType  `json:"type"`
	Description   string    `json:"description,omitempty"`
	// code node fields
	WebhookURL    string    `json:"webhook_url,omitempty"`
	// condition node fields
	Expression    string    `json:"expression,omitempty"`
	// human node fields
	AssigneeGroup string    `json:"assignee_group,omitempty"`
	// llm node fields
	LLMConfig     *LLMConfig `json:"llm_config,omitempty"`
	// agent node fields
	AgentConfig   *AgentConfig `json:"agent_config,omitempty"`
}

// Edge defines a connection between two nodes in a workflow template.
// For condition nodes, OutputPort specifies the branch ("true" or "false").
// Regular nodes and START/END use empty OutputPort.
type Edge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	OutputPort string `json:"output_port,omitempty"` // "true" / "false" for condition nodes
}

// Template defines a workflow blueprint, created by the platform user.
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Nodes       []Node    `json:"nodes"`
	Edges       []Edge    `json:"edges"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// InstanceStatus represents the current status of a workflow instance.
type InstanceStatus string

const (
	StatusPending   InstanceStatus = "pending"
	StatusRunning   InstanceStatus = "running"
	StatusPaused    InstanceStatus = "paused"    // waiting for human input
	StatusCompleted InstanceStatus = "completed"
	StatusFailed    InstanceStatus = "failed"
)

// NodeExecutionState records the result of a single node execution.
type NodeExecutionState struct {
	NodeID   string      `json:"node_id"`
	Status   string      `json:"status"` // "pending", "running", "completed", "failed", "paused"
	Output   interface{} `json:"output,omitempty"`
	Error    string      `json:"error,omitempty"`
}

// Instance represents a single execution of a workflow template.
type Instance struct {
	ID            string                        `json:"id"`
	TemplateID    string                        `json:"template_id"`
	Status        InstanceStatus                `json:"status"`
	State         map[string]interface{}        `json:"state"`          // global workflow state
	NodeStates    map[string]*NodeExecutionState `json:"node_states"`   // per-node execution state
	CurrentNodeID string                        `json:"current_node_id,omitempty"`
	Error         string                        `json:"error,omitempty"`
	CreatedAt     time.Time                     `json:"created_at"`
	UpdatedAt     time.Time                     `json:"updated_at"`
}

// HumanTaskStatus represents the status of a human task.
type HumanTaskStatus string

const (
	HumanTaskPending  HumanTaskStatus = "pending"
	HumanTaskApproved HumanTaskStatus = "approved"
	HumanTaskRejected HumanTaskStatus = "rejected"
)

// HumanTask represents a task that needs human input to proceed.
type HumanTask struct {
	ID              string          `json:"id"`
	InstanceID      string          `json:"instance_id"`
	TemplateID      string          `json:"template_id"`
	NodeID          string          `json:"node_id"`
	NodeDescription string          `json:"node_description"`
	AssigneeGroup   string          `json:"assignee_group"`
	Status          HumanTaskStatus `json:"status"`
	InputData       interface{}     `json:"input_data"`    // state snapshot when paused
	Result          interface{}     `json:"result,omitempty"` // human's input
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// StartInstanceRequest is the request body for starting a new instance.
type StartInstanceRequest struct {
	Input map[string]interface{} `json:"input"`
}

// ResumeTaskRequest is the request body for resuming a human task.
type ResumeTaskRequest struct {
	Action string      `json:"action"` // "approve" or "reject"
	Result interface{} `json:"result"`
}

// CreateTemplateRequest is the request body for creating a template.
type CreateTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Nodes       []Node `json:"nodes"`
	Edges       []Edge `json:"edges"`
}