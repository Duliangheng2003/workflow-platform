# Workflow Platform

[English](README.md) | [中文](README.zh_CN.md)

A personal AI workflow engine built on [Eino](https://github.com/cloudwego/eino). Design workflows with a visual editor, run them with AI-powered agents that can call tools, fetch web data, and execute scripts.

## Features

- **Visual workflow builder** — drag-and-drop nodes on canvas, connect them with edges
- **Dual edge system** — Flow edges for execution order, Data edges for context sharing
- **AI Agent** — uses eino's ReAct agent for multi-step reasoning, knows what tools it has
- **Built-in tools** — `web_search`, `web_fetch`, `now`, `write_file` — available to every Agent
- **Node-based tools** — API Call and Code nodes can be called as tools by the Agent
- **Extractor** — upload files, LLM summarizes them, passes context to Agent via Data edges
- **Thinking trace** — Agent's intermediate thoughts and tool calls are recorded and displayed
- **Code execution** — Code nodes execute JS/Python scripts, callable by Agent as tools

## Node Types

| Type | Label | Border | Description |
|------|-------|--------|-------------|
| `call` | API Call | Blue | HTTP request to external API |
| `agent` | AI Agent | Purple | LLM-powered ReAct agent with tool calling |
| `condition` | Condition | Orange | Expression evaluation for branching |
| `code` | Code | Green | JS/Python script execution |
| `extractor` | Extractor | Cyan | File upload + LLM summarization |

## Edge Types

| Type | Style | Direction | Purpose |
|------|-------|-----------|---------|
| Flow | Solid + arrow | A → B | Execution order + data pipeline |
| Data | Dashed, no arrow | A — B | Context sharing to Agent (only Agent/Extractor can create) |

## Quick Start

### 1. Configure

Edit `config.yaml`:

```yaml
server:
  port: 8080

database:
  # Empty = use in-memory storage (data lost on restart)
  host: ""

llm:
  profiles:
    - name: deepseek-chat
      provider: openai
      model: deepseek-chat
      api_key: sk-your-key
      base_url: "https://api.deepseek.com"
```

### 2. Run

```bash
# Direct run
go run -ldflags=-checklinkname=0 ./cmd/server/

# Build
go build -ldflags=-checklinkname=0 -o workflow-server ./cmd/server/
./workflow-server
```

Open `http://localhost:8080` in your browser.

> Note: `-ldflags=-checklinkname=0` is required for Go 1.24 compatibility with the sonic library.

## API

### Templates

```bash
# Create
curl -X POST http://localhost:8080/api/v1/templates \
  -H "Content-Type: application/json" \
  -d '{"name":"my_workflow","nodes":[{"id":"call_1","type":"call","webhook_url":"https://api.example.com"}],"edges":[{"from":"START","to":"call_1"},{"from":"call_1","to":"END"}]}'

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
curl -X POST http://localhost:8080/api/v1/templates/{id}/instances \
  -H "Content-Type: application/json" \
  -d '{"input": {}}'

# List
curl http://localhost:8080/api/v1/instances

# Detail
curl http://localhost:8080/api/v1/instances/{id}
```

## Project Structure

```
cmd/server/main.go              # Entry point
config.yaml                     # Configuration
internal/
  config/config.go              # Config loader
  model/types.go                # Data models
  store/                        # Storage (memory / MySQL)
  engine/
    engine.go                   # Eino graph builder
    nodes.go                    # Node execution logic
    agent.go                    # Agent + tool wrappers
    tools.go                    # Built-in tools (now, web_fetch, etc.)
    chatmodel.go                # ChatModel wrapper
    llm.go                      # LLM API client
  api/handler.go                # HTTP API handlers
  server/server.go              # Server + embedded static files
  server/static/                # Frontend (HTML/CSS/JS)
```

## Architecture

- Workflow templates are translated into eino `compose.Graph` instances at runtime
- Flow edges build the execution graph, Data edges are skipped (they only provide context)
- Agent nodes use eino's `react` agent for ReAct loop with tool calling
- Built-in tools are automatically available to every Agent
- Thinking trace is collected via `react.WithMessageFuture()` and stored in instance state
- Extractor uses configured LLM profile to summarize uploaded files
- Code nodes execute JS/Python via `node`/`python3` CLI commands