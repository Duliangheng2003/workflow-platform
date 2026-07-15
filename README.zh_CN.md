# 工作流平台

[English](README.md) | [中文](README.zh_CN.md)

基于 [Eino](https://github.com/cloudwego/eino) 构建的个人 AI 工作流引擎。可视化设计工作流，用 AI Agent 执行搜索、API 调用、脚本执行和多步推理任务。

## 功能特性

- **可视化工作流编辑器** — 拖拽节点，用 Flow/Data 边连接，属性面板可调整宽度
- **AI Agent** — 基于 eino ReAct 的多轮推理 Agent，支持工具调用和权限控制
- **内置工具** — `now`、`web_search`、`web_fetch`、`read_file`、`write_file`，按 Agent 独立配置权限
- **节点工具** — API Call 和 Code 节点可作为 Agent 的工具被调用
- **提取器** — 上传文件，LLM 总结后通过 Data 边传给 Agent
- **代码执行** — Code 节点执行 JS/Python 脚本，使用 `data` 变量
- **定时调度** — Schedule 类型模板按 cron 表达式自动运行
- **双边系统** — Flow 边控制执行顺序，Data 边共享上下文
- **撤销/重做** — 支持快捷键 Ctrl+Z / Ctrl+Y
- **模板管理** — 卡片网格布局，悬浮菜单，增删改查
- **实例追踪** — 实时节点状态追踪（pending/running/success/failed）
- **子工作流** — 调用其他已保存的模板作为子流程

## 节点类型

| 类型 | 标签 | 颜色 | 说明 |
|------|------|------|------|
| `call` | API Call | 蓝色 | HTTP 请求外部 API |
| `agent` | AI Agent | 紫色 | LLM ReAct Agent，支持工具调用 |
| `condition` | Condition | 橙色 | IF/ELSE 条件分支 |
| `code` | Code | 绿色 | JS/Python 脚本执行 |
| `extractor` | Extractor | 青色 | 文件上传 + LLM 总结 |
| `filter` | Filter | 青蓝 | 数据过滤/转换 |
| `subworkflow` | Sub-Workflow | 靛蓝 | 调用其他模板作为子流程 |

## 边类型

| 类型 | 说明 |
|------|------|
| `flow` | 执行顺序 — 输出端口 → 输入端口 |
| `data` | 上下文共享 — 数据端口（底部）→ 目标节点 |

## Condition 节点

Condition 节点有两个带标签的输出端口：
- **IF**（绿色）— 表达式为 `true` 时走此分支
- **ELSE**（红色）— 表达式为 `false` 时走此分支

表达式使用 `state.` 前缀，如 `state._global.score >= 60`。支持的操作符：`>=`、`<=`、`>`、`<`、`=`、`!=`。

## Agent 权限

每个 Agent 节点有独立的权限开关：
- **读取本地文件** — 启用 `read_file` 工具（读取/列表/搜索文件）
- **写入本地文件** — 启用 `write_file` 工具
- **网络访问** — 启用 `web_search` + `web_fetch` 工具

## 快速开始

### 前置条件

- Go 1.22+
- Node.js 18+（前端开发用）

### 编译运行

```bash
# 编译前端（可选，已内置静态文件）
cd web && npm install && npm run build && cd ..

# 编译运行服务
go build -ldflags=-checklinkname=0 -o workflow-server ./cmd/server/
./workflow-server
```

打开 http://localhost:8080

### 前端开发

```bash
cd web
npm install
npm run dev   # Vite 开发服务器，端口 5173，API 代理到 :8080
```

### 配置

编辑 `config.yaml`：

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

## 架构

```
workflow-platform/
├── cmd/server/          # 入口
├── internal/
│   ├── api/             # HTTP 处理
│   ├── config/          # 配置加载
│   ├── engine/          # 工作流执行引擎（eino）
│   ├── model/           # 数据类型
│   ├── server/          # HTTP 服务 + 内嵌静态文件
│   └── store/           # SQLite/MySQL/内存存储
├── web/                 # React + TypeScript 前端
│   ├── src/
│   │   ├── pages/       # TemplatesPage, InstancesPage, BuilderPage
│   │   ├── components/  # 可复用 UI 组件
│   │   ├── store.ts     # Zustand 状态管理
│   │   └── types.ts     # TypeScript 类型定义
│   └── dist/            # 构建产物 → 复制到 static/
└── config.yaml          # 服务 + LLM 配置
```

## 技术栈

- **后端**：Go、[Eino](https://github.com/cloudwego/eino)（工作流引擎）
- **前端**：React 18、TypeScript、Vite、Zustand
- **存储**：SQLite（默认）、MySQL（可选）
- **LLM**：OpenAI 兼容 API（DeepSeek、OpenAI 等）
