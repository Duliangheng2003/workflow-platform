# Workflow Platform

基于 [Eino](https://github.com/cloudwego/eino) 框架的通用 AI 工作流引擎平台。

## 核心概念

- **Template（流程模板）**：用户定义的工作流蓝图，包含节点和连接
- **Instance（流程实例）**：模板的一次执行，记录运行状态和共享数据
- **HumanTask（人工任务）**：需要人工介入时暂停，等待审批后恢复

## 节点类型

| 类型 | 说明 |
|------|------|
| `code` | HTTP 回调节点，调用用户配置的 webhook |
| `condition` | 条件判断节点，基于表达式走不同分支 |
| `human` | 人工节点，暂停流程等待人工输入 |

## 快速开始

```bash
# 启动服务
go run ./cmd/server

# 服务运行在 :8080
```

## API 示例

### 1. 创建流程模板

```bash
curl -X POST http://localhost:8080/api/v1/templates \
  -H "Content-Type: application/json" \
  -d '{
    "name": "review_flow",
    "description": "A simple review workflow",
    "nodes": [
      {"id": "validate", "type": "code", "webhook_url": "http://localhost:9000/validate"},
      {"id": "approve", "type": "human", "description": "Please review the data"},
      {"id": "check", "type": "condition", "expression": "state.approve.approved == true"},
      {"id": "process", "type": "code", "webhook_url": "http://localhost:9000/process"},
      {"id": "reject", "type": "code", "webhook_url": "http://localhost:9000/reject"}
    ],
    "edges": [
      {"from": "START", "to": "validate"},
      {"from": "validate", "to": "approve"},
      {"from": "approve", "to": "check"},
      {"from": "check", "to": "process", "output_port": "true"},
      {"from": "check", "to": "reject", "output_port": "false"},
      {"from": "process", "to": "END"},
      {"from": "reject", "to": "END"}
    ]
  }'
```

### 2. 启动流程实例

```bash
curl -X POST http://localhost:8080/api/v1/templates/{template_id}/instances \
  -H "Content-Type: application/json" \
  -d '{"input": {"order_id": "12345", "amount": 1000}}'
```

### 3. 查看待处理的人工任务

```bash
curl http://localhost:8080/api/v1/human-tasks?status=pending
```

### 4. 审批人工任务

```bash
curl -X POST http://localhost:8080/api/v1/human-tasks/{task_id}/resume \
  -H "Content-Type: application/json" \
  -d '{"action": "approve", "result": {"comment": "Looks good"}}'
```

## 项目结构

```
cmd/server/main.go        # 服务入口
internal/
  model/types.go          # 数据模型
  store/interface.go       # 存储接口
  store/memory.go          # 内存存储实现
  engine/engine.go         # 工作流执行引擎
  api/handler.go           # HTTP API 处理
  server/server.go         # 服务启动
```