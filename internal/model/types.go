package model

import "time"

// NodeType defines the type of a workflow node.
type NodeType string

const (
	NodeTypeCode      NodeType = "code"
	NodeTypeCondition NodeType = "condition"
	NodeTypeHuman     NodeType = "human"
)

// Node defines a single node in a workflow template.
type Node struct {
	ID            string   `json:"id"`
	Type          NodeType `json:"type"`
	WebhookURL    string   `json:"webhook_url,omitempty"`    // for code node
	AssigneeGroup string   `json:"assignee_group,omitempty"` // for human node
	Expression    string   `json:"expression,omitempty"`     // for condition node
	Description   string   `json:"description,omitempty"`
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