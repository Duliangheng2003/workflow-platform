# Workflow Platform

[English](README.md) | [中文](README.zh_CN.md)

基于 [Eino](https://github.com/cloudwego/eino) 的个人 AI 工作流引擎。通过可视化编辑器设计工作流，使用 AI Agent 执行多轮推理、搜索网页、调用 API、执行脚本。

## 功能特性

- **可视化流程编辑器** — 拖拽节点到画布，用 Flow/Data 边连接，右侧面板配置属性
- **AI Agent** — eino ReAct 智能代理，多轮推理，知道自身有哪些工具可用
- **内置工具** — `web_search`、`web_fetch`、`now`、`write_file`，每个 Agent 自动拥有
- **节点工具** — API Call 和 Code 节点可被 Agent 作为工具调用
- **提取器** — 上传文件，LLM 自动总结，通过 Data 边传给 Agent
- **实时思考过程** — Agent 的中间推理步骤和工具调用实时展示
- **代码执行** — Code 节点执行 JS/Python 脚本
- **定时调度** — Schedule 类型模板按 cron 表达式自动执行
- **双类型连线** — Flow 边控制执行顺序，Data 边传递业务上下文

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

## 技术栈

| 层 | 技术 |
|----|------|
| 前端 | 原生 HTML/CSS/JS（无框架） |
| 后端 | Go 1.24，[Eino](https://github.com/cloudwego/eino)（ReAct Agent、图执行引擎） |
| 数据库 | SQLite（默认），支持 MySQL，可回退到内存存储 |
| LLM | 兼容 OpenAI API 格式（OpenAI、DeepSeek 等） |
| 存储层 | `database/sql` + go-sqlite3 / go-sql-driver-mysql |

## 快速开始

### 1. 配置

编辑 `config.yaml`：

```yaml
server:
  port: 8080

database:
  path: "workflow.db"  # SQLite 文件路径，留空则使用内存存储

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
go run -ldflags=-checklinkname=0 ./cmd/server/
```

浏览器打开 `http://localhost:8080`。

> 注意：Go 1.24 需要 `-ldflags=-checklinkname=0` 参数解决 Eino 依赖的 sonic 库兼容性问题。

## 项目结构

```
cmd/server/main.go              # 程序入口
config.yaml                     # 配置文件
internal/
  config/config.go              # 配置加载
  model/types.go                # 数据模型
  store/                        # 存储层（sqlite / mysql / 内存）
  engine/
    engine.go                   # Eino 图构建 + 定时调度
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
- 思考过程通过 `react.WithMessageFuture()` 收集，前端轮询实时展示
- Extractor 使用配置的 LLM profile 对上传文件进行总结
- Code 节点通过 `node`/`python3` 命令执行脚本
- 定时调度器每分钟检查一次 Schedule 类型模板的 cron 表达式

---

> **注意：** 此项目仍在积极开发中，功能可能会有变化，部分功能仍在完善。