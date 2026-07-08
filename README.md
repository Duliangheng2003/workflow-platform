# Workflow Platform

A general-purpose AI workflow engine built on [Eino](https://github.com/cloudwego/eino).

## Core Concepts

- **Template**: A workflow blueprint defined by the user, containing nodes and edges.
- **Instance**: A single execution of a template, tracking state and node results.
- **HumanTask**: A pause point in the workflow where human input is required.

## Node Types

| Type | Description |
|------|-------------|
| `code` | HTTP callback — calls a user-configured webhook with current state |
| `llm` | LLM call — invokes an OpenAI-compatible chat model |
| `condition` | Conditional branch — routes execution based on `state.path.to.value = expected` |
| `human` | Human task — pauses the workflow until approved/rejected via API |

## Quick Start

```bash
go run ./cmd/server
# Server starts on :8080 — open http://localhost:8080 in your browser
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
      {"id": "approve","type": "human", "description": "Review the result"},
      {"id": "process", "type": "code", "webhook_url": "http://my-app/process"},
      {"id": "reject",  "type": "llm", "llm_config": {
        "provider": "openai", "model_name": "gpt-4o",
        "api_key_env": "OPENAI_API_KEY",
        "user_prompt": "Generate a rejection notice for: {state._global}"
      }}
    ],
    "edges": [
      {"from":"START","to":"validate"},
      {"from":"validate","to":"check"},
      {"from":"check","to":"approve","output_port":"true"},
      {"from":"check","to":"reject","output_port":"false"},
      {"from":"approve","to":"process"},
      {"from":"process","to":"END"},
      {"from":"reject","to":"END"}
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
# Start
curl -X POST http://localhost:8080/api/v1/templates/{template_id}/instances \
  -H "Content-Type: application/json" \
  -d '{"input": {"order_id": "12345"}}'

# List
curl http://localhost:8080/api/v1/instances

# Get
curl http://localhost:8080/api/v1/instances/{id}
```

### Human Tasks

```bash
# List (all or filter by status)
curl http://localhost:8080/api/v1/human-tasks?status=pending

# Approve / Reject
curl -X POST http://localhost:8080/api/v1/human-tasks/{task_id}/resume \
  -H "Content-Type: application/json" \
  -d '{"action": "approve", "result": {"comment": "Looks good"}}'
```

## Project Structure

```
cmd/server/main.go        # Entry point
internal/
  model/types.go          # Data models
  store/interface.go       # Store interface
  store/memory.go          # In-memory implementation
  engine/engine.go         # Eino graph builder & runner
  engine/nodes.go          # Node lambda implementations
  engine/llm.go            # LLM API client
  api/handler.go           # HTTP API handlers
  server/server.go         # Server bootstrap + embedded frontend
  server/static/           # Frontend HTML/CSS/JS
```

## Architecture

The engine translates user-defined workflow templates into [Eino](https://github.com/cloudwego/eino) `compose.Graph` instances at runtime:

- `code` / `llm` / `human` nodes → `LambdaNode`
- `condition` nodes → `GraphBranch` on the predecessor node
- State flows as `map[string]any` through the graph
- Human nodes use a channel-based interrupt/resume pattern