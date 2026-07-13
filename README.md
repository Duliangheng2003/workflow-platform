# Workflow Platform

[English](README.md) | [中文](README.zh_CN.md)

A personal AI workflow engine built on [Eino](https://github.com/cloudwego/eino). Design workflows visually, run them with AI-powered agents that can search the web, call APIs, execute scripts, and reason through multi-step tasks.

## Features

- **Visual workflow builder** — drag-and-drop nodes, connect with Flow/Data edges, configure in the right panel
- **AI Agent** — eino ReAct agent with multi-step reasoning, knows what tools it has
- **Built-in tools** — `web_search`, `web_fetch`, `now`, `write_file` — every Agent can use them
- **Node-based tools** — API Call and Code nodes can be called as tools by the Agent
- **Extractor** — upload files, LLM summarizes, passes context to Agent via Data edges
- **Real-time thinking trace** — Agent's intermediate thoughts and tool calls are displayed live
- **Code execution** — Code nodes execute JS/Python scripts
- **Cron scheduling** — Schedule-type templates run automatically on cron expressions
- **Dual edge system** — Flow edges for execution order, Data edges for context sharing

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
| Data | Dashed, no arrow | A — B | Context sharing (only Agent/Extractor can create) |

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Frontend | Vanilla HTML/CSS/JS (no framework) |
| Backend | Go 1.24, [Eino](https://github.com/cloudwego/eino) (ReAct agent, graph execution) |
| Database | SQLite (default), MySQL supported, in-memory fallback |
| LLM | OpenAI-compatible APIs (OpenAI, DeepSeek, etc.) |
| Storage | `database/sql` + go-sqlite3 / go-sql-driver-mysql |

## Quick Start

### 1. Configure

Edit `config.yaml`:

```yaml
server:
  port: 8080

database:
  path: "workflow.db"  # SQLite file, or leave empty for in-memory

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
go run -ldflags=-checklinkname=0 ./cmd/server/
```

Open `http://localhost:8080` in your browser.

> Note: `-ldflags=-checklinkname=0` is required for Go 1.24 compatibility with the sonic library used by Eino.

## Project Structure

```
cmd/server/main.go              # Entry point
config.yaml                     # Configuration
internal/
  config/config.go              # Config loader
  model/types.go                # Data models
  store/                        # Storage (sqlite / mysql / memory)
  engine/
    engine.go                   # Eino graph builder + cron scheduler
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
- Flow edges build the execution graph; Data edges are skipped (they only provide context)
- Agent nodes use eino's `react` agent for ReAct loop with tool calling
- Built-in tools are automatically available to every Agent
- Thinking trace is collected via `react.WithMessageFuture()` and polled by the frontend in real-time
- Extractor uses configured LLM profile to summarize uploaded files
- Code nodes execute JS/Python via `node`/`python3` CLI commands
- Cron scheduler runs every minute, checks Schedule-type templates for matching cron expressions

---

> **Note:** This project is actively under development. Features may change, and some functionality is still being refined.