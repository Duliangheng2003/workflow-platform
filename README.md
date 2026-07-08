# Workflow Platform

[English](README.md) | [中文](README.zh_CN.md)

A general-purpose AI workflow engine built on [Eino](https://github.com/cloudwego/eino).

## Core Concepts

- **Template**: A workflow blueprint defined by the user, containing nodes and edges.
- **Instance**: A single execution of a template, tracking state and node results.
- **HumanTask**: A pause point where human input is required before continuing.

## Node Types

| Type | Description |
|------|-------------|
| `code` | HTTP callback — calls a user-configured webhook with current state |
| `agent` | AI Agent — uses eino's ReAct loop to autonomously call tools (other nodes) |
| `condition` | Conditional branch — routes based on `state.path.to.value = expected` |
| `human` | Human task — pauses the workflow until approved/rejected |

## Quick Start

### 1. Configure

Copy and edit `config.yaml`:

```yaml
server:
  port: 8080

database:
  host: 127.0.0.1
  port: 3306
  user: root
  password: your_password
  database: workflow_platform

llm:
  profiles:
    - name: openai-gpt4o
      provider: openai
      model: gpt-4o
      api_key: sk-your-key
      base_url: "https://api.openai.com"
```

> **Note**: If `database.host` is empty, the server uses in-memory storage (data lost on restart).

### 2. Build & Run

```bash
# Build (required due to sonic library compatibility)
go build -ldflags="-checklinkname=0" -o workflow-platform ./cmd/server

# Or run directly
go run -ldflags="-checklinkname=0" ./cmd/server

# Specify a custom config path
go run -ldflags="-checklinkname=0" ./cmd/server --config /path/to/config.yaml
```

The server starts on `http://localhost:8080`. Open it in your browser to see the dashboard.

### 3. Build for production

```bash
# Cross-compile for Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-checklinkname=0" -o workflow-platform ./cmd/server

# Run
./workflow-platform --config config.yaml
```

## API

### Templates

```bash
# Create
curl -X POST http://localhost:8080/api/v1/templates \
  -H "Content-Type: application/json" \
  -d '{
    "name": "review_flow",
    "nodes": [
      {"id": "validate", "type": "code", "webhook_url": "http://my-app/validate"},
      {"id": "check",  "type": "condition", "expression": "state.validate.result == ok"},
      {"id": "process", "type": "code", "webhook_url": "http://my-app/process"}
    ],
    "edges": [
      {"from":"START","to":"validate"},
      {"from":"validate","to":"check"},
      {"from":"check","to":"process","output_port":"true"},
      {"from":"check","to":"END","output_port":"false"},
      {"from":"process","to":"END"}
    ]
  }'

# List
curl http://localhost:8080/api/v1/templates

# Get
curl http://localhost:8080/api/v1/templates/{id}

# Delete
curl -X DELETE http://localhost:8080/api/v1/templates/{id}
```

### Instances

```bash
# Start a new instance
curl -X POST http://localhost:8080/api/v1/templates/{template_id}/instances \
  -H "Content-Type: application/json" \
  -d '{"input": {"order_id": "12345"}}'

# List
curl http://localhost:8080/api/v1/instances

# Get detail
curl http://localhost:8080/api/v1/instances/{id}
```

### Human Tasks

```bash
# List (all or filter by status)
curl http://localhost:8080/api/v1/human-tasks?status=pending

# Approve
curl -X POST http://localhost:8080/api/v1/human-tasks/{task_id}/resume \
  -H "Content-Type: application/json" \
  -d '{"action": "approve", "result": {"comment": "Looks good"}}'

# Reject
curl -X POST http://localhost:8080/api/v1/human-tasks/{task_id}/resume \
  -H "Content-Type: application/json" \
  -d '{"action": "reject", "result": {"reason": "Invalid request"}}'
```

## Configuration

### Database

| Field | Description |
|-------|-------------|
| `host` | MySQL host (empty = use in-memory store) |
| `port` | MySQL port (default: 3306) |
| `user` | MySQL user |
| `password` | MySQL password |
| `database` | Database name |

Tables are created automatically on startup.

### LLM Profiles

API keys are stored server-side — never exposed to the frontend:

```yaml
llm:
  profiles:
    - name: openai-gpt4o
      provider: openai
      model: gpt-4o
      api_key: sk-xxx
      base_url: "https://api.openai.com"
    - name: deepseek-chat
      provider: deepseek
      model: deepseek-chat
      api_key: sk-xxx
      base_url: "https://api.deepseek.com"
```

## Project Structure

```
cmd/server/main.go              # Entry point
internal/
  config/config.go               # Config loader
  model/types.go                 # Data models
  store/interface.go             # Store interface
  store/memory.go                # In-memory store
  store/mysql/mysql.go           # MySQL store
  engine/engine.go               # Eino graph builder & runner
  engine/nodes.go                # Code / condition / human node lambdas
  engine/agent.go                # Agent node (ReAct loop + tool wrapping)
  engine/chatmodel.go            # ChatModel wrapper (BaseChatModel impl)
  engine/llm.go                  # LLM API client
  api/handler.go                 # HTTP API handlers
  server/server.go               # Server bootstrap + embedded frontend
  server/static/                 # Frontend HTML/CSS/JS
config.yaml                      # Configuration file
```

## Architecture

The engine translates workflow templates into [Eino](https://github.com/cloudwego/eino) `compose.Graph` instances at runtime:

- `code` / `agent` / `human` nodes → `LambdaNode`
- `condition` nodes → `GraphBranch` on the predecessor node
- State flows as `map[string]any` through the graph
- Agent nodes execute a ReAct loop (ChatModelAgent) that can call other nodes as tools
- Human nodes use a channel-based interrupt/resume pattern
- API keys are stored in `config.yaml`, never exposed to the frontend