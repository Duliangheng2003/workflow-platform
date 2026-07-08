# Workflow Platform

[English](README.md) | [中文](README.zh_CN.md)

基于 [Eino](https://github.com/cloudwego/eino) 的通用 AI 工作流引擎平台。

## 核心概念

- **Template（流程模板）**：用户定义的工作流蓝图，包含节点和连接
- **Instance（流程实例）**：模板的一次执行，记录运行状态和节点结果
- **HumanTask（人工任务）**：流程中需要人工介入的暂停点

## 节点类型

| 类型 | 说明 |
|------|------|
| `code` | HTTP 回调 — 调用用户配置的 webhook，传入当前 state |
| `agent` | AI 智能体 — 基于 eino ReAct 循环，能自主调用其他节点作为工具 |
| `condition` | 条件判断 — 基于 `state.path.to.value = expected` 表达式路由 |
| `human` | 人工节点 — 暂停流程，等待审批后继续 |

## 快速开始

### 1. 配置

复制 `config.yaml` 并编辑：

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

> **注意**：如果 `database.host` 为空，服务将使用内存存储（重启后数据丢失）。

### 2. 编译与启动

```bash
# 编译（需要 ldflags 处理 sonic 库兼容性）
go build -ldflags="-checklinkname=0" -o workflow-platform ./cmd/server

# 或直接运行
go run -ldflags="-checklinkname=0" ./cmd/server

# 指定配置文件路径
go run -ldflags="-checklinkname=0" ./cmd/server --config /path/to/config.yaml
```

服务启动后访问 `http://localhost:8080` 即可看到管理界面。

### 3. 生产部署

```bash
# 交叉编译 Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-checklinkname=0" -o workflow-platform ./cmd/server

# 运行
./workflow-platform --config config.yaml
```

## API 接口

### 流程模板

```bash
# 创建模板
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

# 列表
curl http://localhost:8080/api/v1/templates

# 详情
curl http://localhost:8080/api/v1/templates/{id}

# 删除
curl -X DELETE http://localhost:8080/api/v1/templates/{id}
```

### 流程实例

```bash
# 启动实例
curl -X POST http://localhost:8080/api/v1/templates/{template_id}/instances \
  -H "Content-Type: application/json" \
  -d '{"input": {"order_id": "12345"}}'

# 列表
curl http://localhost:8080/api/v1/instances

# 详情
curl http://localhost:8080/api/v1/instances/{id}
```

### 人工任务

```bash
# 查看待办（可按状态筛选）
curl http://localhost:8080/api/v1/human-tasks?status=pending

# 审批通过
curl -X POST http://localhost:8080/api/v1/human-tasks/{task_id}/resume \
  -H "Content-Type: application/json" \
  -d '{"action": "approve", "result": {"comment": "通过"}}'

# 拒绝
curl -X POST http://localhost:8080/api/v1/human-tasks/{task_id}/resume \
  -H "Content-Type: application/json" \
  -d '{"action": "reject", "result": {"reason": "信息有误"}}'
```

## 配置说明

### 数据库

| 字段 | 说明 |
|------|------|
| `host` | MySQL 地址（为空则使用内存存储） |
| `port` | MySQL 端口（默认 3306） |
| `user` | MySQL 用户名 |
| `password` | MySQL 密码 |
| `database` | 数据库名 |

启动时自动创建表结构。

### LLM 配置

API Key 仅存储在服务端，前端不可见：

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

## 项目结构

```
cmd/server/main.go              # 入口
internal/
  config/config.go               # 配置加载
  model/types.go                 # 数据模型
  store/interface.go             # 存储接口
  store/memory.go                # 内存存储
  store/mysql/mysql.go           # MySQL 存储
  engine/engine.go               # Eino 图构建器
  engine/nodes.go                # code / condition / human 节点执行器
  engine/agent.go                # Agent 节点（ReAct 循环 + 工具包装）
  engine/chatmodel.go            # ChatModel 包装器
  engine/llm.go                  # LLM API 客户端
  api/handler.go                 # HTTP API 处理
  server/server.go               # 服务启动 + 嵌入式前端
  server/static/                 # 前端页面
config.yaml                      # 配置文件
```

## 架构说明

运行时将用户定义的流程模板转换为 [Eino](https://github.com/cloudwego/eino) `compose.Graph`：

- `code` / `agent` / `human` 节点 → `LambdaNode`
- `condition` 节点 → 前驱节点上的 `GraphBranch`
- State 以 `map[string]any` 形式在图中传递
- Agent 节点内部运行 ReAct 循环，可调用其他节点作为工具
- 人工节点通过 channel 实现中断/恢复
- API Key 配置在 `config.yaml` 中，前端不可见