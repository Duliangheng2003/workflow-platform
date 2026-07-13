# Workflow Platform

[English](README.md) | [中文](README.zh_CN.md)

基于 [Eino](https://github.com/cloudwego/eino) 的个人 AI 工作流引擎。通过可视化编辑器设计工作流，使用 AI Agent 执行多轮推理、调用工具、抓取网页、执行脚本。

## 功能特性

- **可视化流程编辑器** — 拖拽节点到画布，用连线构建流程
- **双类型连线** — Flow 边控制执行顺序，Data 边传递业务上下文
- **AI Agent** — 基于 eino ReAct 实现多轮推理，知道自身有哪些工具可用
- **内置工具** — `web_search`、`web_fetch`、`now`、`write_file`，每个 Agent 自动拥有
- **节点工具** — API Call 和 Code 节点可被 Agent 作为工具调用
- **提取器** — 上传文件，LLM 自动总结，通过 Data 边传给 Agent
- **思考过程** — Agent 的中间推理步骤和工具调用会被记录并展示
- **代码执行** — Code 节点执行 JS/Python 脚本，可被 Agent 调用

## 节点类型

| 类型 | 标签 | 边框色 | 说明 |
|------|------|--------|------|
| `call` | API Call | 蓝色 | 调用外部 HTTP API |
| `agent` | AI Agent | 紫色 | LLM 驱动的 ReAct 智能代理 |
| `condition` | Condition | 橙色 | 表达式求值，控制流程分支 |
| `code` | Code | 绿色 | 执行 JS/Python 脚本 |
| `extractor` | Extractor | 青色 | 上传文件 + LLM 自动总结 |

## 边类型

| 类型 | 样式 | 方向 | 用途 |
|------|------|------|------|
| Flow | 实线+箭头 | A → B | 执行顺序 + 数据管道 |
| Data | 虚线，无箭头 | A — B | 向 Agent 传递上下文（仅 Agent/Extractor 可创建） |

## 快速开始

### 1. 配置

编辑 `config.yaml`：

```yaml
server:
  port: 8080

database:
  # 默认使用 MySQL。留空 host 则使用内存存储（重启后数据丢失）。
  host: "127.0.0.1"
  port: 3306
  user: "root"
  password: "your_password"
  database: "workflow_platform"

llm:
  profiles:
    - name: deepseek-chat
      provider: openai
      model: deepseek-chat
      api_key: sk-your-key
      base_url: "https://api.deepseek.com"
```

### 2. 启动

```bash
# 直接运行
go run -ldflags=-checklinkname=0 ./cmd/server/

# 编译
go build -ldflags=-checklinkname=0 -o workflow-server ./cmd/server/
./workflow-server
```

浏览器打开 `http://localhost:8080`。

> 注意：Go 1.24 需要 `-ldflags=-checklinkname=0` 参数解决 sonic 库兼容性问题。

## API 接口

### 模板管理

```bash
# 创建
curl -X POST http://localhost:8080/api/v1/templates \
  -H "Content-Type: application/json" \
  -d '{"name":"my_workflow","nodes":[{"id":"call_1","type":"call","webhook_url":"https://api.example.com"}],"edges":[{"from":"START","to":"call_1"},{"from":"call_1","to":"END"}]}'

# 列表
curl http://localhost:8080/api/v1/templates

# 详情
curl http://localhost:8080/api/v1/templates/{id}

# 删除
curl -X DELETE http://localhost:8080/api/v1/templates/{id}
```

### 实例管理

```bash
# 启动实例
curl -X POST http://localhost:8080/api/v1/templates/{id}/instances \
  -H "Content-Type: application/json" \
  -d '{"input": {}}'

# 列表
curl http://localhost:8080/api/v1/instances

# 详情
curl http://localhost:8080/api/v1/instances/{id}
```

## 项目结构

```
cmd/server/main.go              # 程序入口
config.yaml                     # 配置文件
internal/
  config/config.go              # 配置加载
  model/types.go                # 数据模型
  store/                        # 存储层（内存 / MySQL）
  engine/
    engine.go                   # Eino 图构建
    nodes.go                    # 节点执行逻辑
    agent.go                    # Agent + 工具包装
    tools.go                    # 内置工具（now, web_fetch 等）
    chatmodel.go                # ChatModel 包装器
    llm.go                      # LLM API 客户端
  api/handler.go                # HTTP API 处理
  server/server.go              # 服务启动 + 嵌入式前端
  server/static/                # 前端页面（HTML/CSS/JS）
```

## 架构说明

- 流程模板在运行时转换为 eino `compose.Graph`
- Flow 边构建执行图，Data 边不参与图构建（仅提供上下文）
- Agent 使用 eino 的 `react` 包实现 ReAct 循环
- 内置工具自动注册到每个 Agent
- 思考过程通过 `react.WithMessageFuture()` 收集，存入实例状态
- Extractor 使用配置的 LLM profile 对上传文件进行总结
- Code 节点通过 `node`/`python3` 命令执行脚本