package model

import "time"

// NodeType defines the type of a workflow node.
type NodeType string

const (
	NodeTypeCall      NodeType = "call"      // API Call (HTTP request)
	NodeTypeCondition NodeType = "condition" // Condition expression
	NodeTypeHuman     NodeType = "human"     // Human approval (deprecated)
	NodeTypeLLM       NodeType = "llm"       // LLM call
	NodeTypeAgent     NodeType = "agent"     // AI Agent
	NodeTypeCode      NodeType = "code"      // Code script (JS/Python)
	NodeTypeExtractor NodeType = "extractor" // File data extractor
)

// LLMConfig defines configuration for an LLM node.
type LLMConfig struct {
	Profile      string  `json:"profile"`
	SystemPrompt string  `json:"system_prompt"`
	UserPrompt   string  `json:"user_prompt"`
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
}

// AgentConfig defines configuration for an agent node.
type AgentConfig struct {
	Profile        string   `json:"profile"`
	SystemPrompt   string   `json:"system_prompt"`
	Tools          []string `json:"tools"`
	MaxTurns       int      `json:"max_turns"`
	EnableReadTools  bool `json:"enable_read_tools,omitempty"`
	EnableWriteTools bool `json:"enable_write_tools,omitempty"`
	EnableWebTools   bool `json:"enable_web_tools,omitempty"`
}

// Node defines a single node in a workflow template.
type Node struct {
	ID          string   `json:"id"`
	Type        NodeType `json:"type"`
	Description string   `json:"description,omitempty"`
	// call node fields (API Call)
	WebhookURL  string `json:"webhook_url,omitempty"`
	Method      string `json:"method,omitempty"`
	BodyType    string `json:"body_type,omitempty"`
	BodyContent string `json:"body_content,omitempty"`
	// condition node fields
	Expression string `json:"expression,omitempty"`
	// human node fields
	AssigneeGroup string `json:"assignee_group,omitempty"`
	// llm node fields
	LLMConfig *LLMConfig `json:"llm_config,omitempty"`
	// agent node fields
	AgentConfig *AgentConfig `json:"agent_config,omitempty"`
	// code node fields
	Language string `json:"language,omitempty"` // "js" or "python"
	Code     string `json:"code,omitempty"`     // script content
	// extractor node fields
	FileContent    string `json:"file_content,omitempty"`
	FileName       string `json:"file_name,omitempty"`
	ExtractPrompt  string `json:"extract_prompt,omitempty"`
	LLMProfile     string `json:"llm_profile,omitempty"` // LLM profile for extraction
}

// EdgeType defines the type of a workflow edge.
type EdgeType string

const (
	EdgeTypeFlow EdgeType = "flow"
	EdgeTypeData EdgeType = "data"
)

// Edge defines a connection between two nodes in a workflow template.
type Edge struct {
	From       string   `json:"from"`
	To         string   `json:"to"`
	EdgeType   EdgeType `json:"edge_type,omitempty"`
	OutputPort string   `json:"output_port,omitempty"`
}

// Template defines a workflow blueprint.
type Template struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Nodes       []Node     `json:"nodes"`
	Edges       []Edge     `json:"edges"`
	StartType   string     `json:"start_type,omitempty"`
	CronExpr    string     `json:"cron_expr,omitempty"`
		StartInput  string `json:"start_input,omitempty"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// InstanceStatus represents the current status of a workflow instance.
type InstanceStatus string

const (
	StatusPending   InstanceStatus = "pending"
	StatusRunning   InstanceStatus = "running"
	StatusPaused    InstanceStatus = "paused"
	StatusCompleted InstanceStatus = "completed"
	StatusFailed    InstanceStatus = "failed"
)

// NodeExecutionState records the result of a single node execution.
type NodeExecutionState struct {
	NodeID string      `json:"node_id"`
	Status string      `json:"status"`
	Output interface{} `json:"output,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// Instance represents a single execution of a workflow template.
type Instance struct {
	ID            string                         `json:"id"`
	TemplateID    string                         `json:"template_id"`
	Status        InstanceStatus                 `json:"status"`
	State         map[string]interface{}         `json:"state"`
	NodeStates    map[string]*NodeExecutionState `json:"node_states"`
	CurrentNodeID string                         `json:"current_node_id,omitempty"`
	Error         string                         `json:"error,omitempty"`
	CreatedAt     time.Time                      `json:"created_at"`
	UpdatedAt     time.Time                      `json:"updated_at"`
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
	InputData       interface{}     `json:"input_data"`
	Result          interface{}     `json:"result,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// StartInstanceRequest is the request body for starting a new instance.
type StartInstanceRequest struct {
	Input map[string]interface{} `json:"input"`
}

// ResumeTaskRequest is the request body for resuming a human task.
type ResumeTaskRequest struct {
	Action string      `json:"action"`
	Result interface{} `json:"result"`
}

// CreateTemplateRequest is the request body for creating a template.
type CreateTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Nodes       []Node `json:"nodes"`
	Edges       []Edge `json:"edges"`
	StartType   string `json:"start_type,omitempty"`
	CronExpr    string `json:"cron_expr,omitempty"`
		StartInput  string `json:"start_input,omitempty"`
}