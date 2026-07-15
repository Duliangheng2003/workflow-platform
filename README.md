# Workflow Platform

[English](README.md) | [中文](README.zh_CN.md)

A personal AI workflow engine built on [Eino](https://github.com/cloudwego/eino). Design workflows visually, run them with AI-powered agents that can search the web, call APIs, execute scripts, and reason through multi-step tasks.

## Features

- **Visual workflow builder** — drag-and-drop nodes, connect with Flow/Data edges, resizable properties panel
- **AI Agent** — eino ReAct agent with multi-step reasoning, tool calling, and permission controls
- **Built-in tools** — `now`, `web_search`, `web_fetch`, `read_file`, `write_file` — toggle permissions per agent
- **Node-based tools** — API Call and Code nodes can be called as tools by the Agent
- **Extractor** — upload files, LLM summarizes, passes context to Agent via Data edges
- **Code execution** — Code nodes execute JS/Python scripts with `data` variable
- **Cron scheduling** — Schedule-type templates run automatically on cron expressions
- **Dual edge system** — Flow edges for execution order, Data edges for context sharing
- **Undo/Redo** — Full undo/redo support with keyboard shortcuts (Ctrl+Z / Ctrl+Y)
- **Template management** — Card grid UI with hover menu, create/edit/delete templates
- **Instance tracking** — Real-time node state tracking (pending/running/success/failed)
- **Sub-workflow** — Call other saved templates as sub-processes

## Node Types

| Type | Label | Color | Description |
|------|-------|-------|-------------|
| `call` | API Call | Blue | HTTP request to external API |
| `agent` | AI Agent | Purple | LLM-powered ReAct agent with tool calling |
| `condition` | Condition | Orange | IF/ELSE branching with expression evaluation |
| `code` | Code | Green | JS/Python script execution |
| `extractor` | Extractor | Cyan | File upload + LLM summarization |
| `filter` | Filter | Teal | Data filtering/transformation |
| `subworkflow` | Sub-Workflow | Indigo | Call another template as a sub-process |

## Edge Types

| Type | Description |
|------|-------------|
| `flow` | Execution order — output port → input port |
| `data` | Context sharing — data port (bottom) → target node |

## Condition Node

Condition nodes have two labeled output ports:
- **IF** (green) — route taken when expression is `true`
- **ELSE** (red) — route taken when expression is `false`

Expressions use `state.` prefix, e.g. `state._global.score >= 60`. Supported operators: `>=`, `<=`, `>`, `<`, `=`, `!=`.

## Agent Permissions

Each Agent node has independent permission toggles:
- **Read local files** — enables `read_file` tool (read/list/search files)
- **Write local files** — enables `write_file` tool
- **Web access** — enables `web_search` + `web_fetch` tools

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 18+ (for frontend development)

### Build & Run

```bash
# Build frontend (optional, static files already included)
cd web && npm install && npm run build && cd ..

# Build and run server
go build -ldflags=-checklinkname=0 -o workflow-server ./cmd/server/
./workflow-server
```

Open http://localhost:8080

### Frontend Development

```bash
cd web
npm install
npm run dev   # Vite dev server on port 5173, proxies API to :8080
```

### Configuration

Edit `config.yaml`:

```yaml
server:
  port: 8080

database:
  path: "workflow.db"    # SQLite

llm:
  profiles:
    - name: my-llm
      provider: openai
      model: gpt-4o
      api_key: sk-xxx
      base_url: "https://api.openai.com"
```

## Architecture

```
workflow-platform/
├── cmd/server/          # Entry point
├── internal/
│   ├── api/             # HTTP handlers
│   ├── config/          # Config loading
│   ├── engine/          # Workflow execution engine (eino)
│   ├── model/           # Data types
│   ├── server/          # HTTP server + embedded static files
│   └── store/           # SQLite/MySQL/Memory storage
├── web/                 # React + TypeScript frontend
│   ├── src/
│   │   ├── pages/       # TemplatesPage, InstancesPage, BuilderPage
│   │   ├── components/  # Reusable UI components
│   │   ├── store.ts     # Zustand state management
│   │   └── types.ts     # TypeScript type definitions
│   └── dist/            # Build output → copied to static/
└── config.yaml          # Server + LLM configuration
```

## Tech Stack

- **Backend**: Go, [Eino](https://github.com/cloudwego/eino) (workflow engine)
- **Frontend**: React 18, TypeScript, Vite, Zustand
- **Storage**: SQLite (default), MySQL (optional)
- **LLM**: OpenAI-compatible API (DeepSeek, OpenAI, etc.)
