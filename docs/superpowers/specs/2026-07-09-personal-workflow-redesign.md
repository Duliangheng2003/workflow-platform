# Personal Workflow Platform Redesign

## 概述

将原有团队协作的工作流平台改造为个人使用，聚焦自动化数据处理和 AI 驱动场景。重新设计节点类型、连接关系和数据流机制。

## 节点类型

### API Call (HTTP 请求)

- **用途**：调用外部 HTTP API，合并原通知功能
- **配置项**：Method, URL, Body, Headers
- **输入**：上游节点数据（通过 `{上游ID.output}` 引用）
- **输出**：HTTP 响应体（JSON）
- **典型场景**：抓取网页数据、调用第三方 API、发送通知（Telegram/邮件/钉钉等）

### AI Agent (AI 代理)

- **用途**：智能处理节点，使用 LLM 驱动的 ReAct 循环
- **配置项**：LLM 配置、System Prompt
- **输入**：上游节点数据（自动作为上下文）
- **输出**：结构化结果（内容 + 工具调用记录）
- **工具发现**：自动将所有无向边连接的节点暴露为工具
- **典型场景**：数据分析、文本处理、决策建议

### Condition (条件判断)

- **用途**：表达式求值，决定流程分支
- **配置项**：表达式（如 `state._global.status == ok`）
- **输入**：上游节点数据
- **输出**：true/false 布尔值
- **典型场景**：根据条件走不同分支

### Code/Transform (代码转换)

- **用途**：JS/Python 脚本，数据转换、清洗、格式化
- **配置项**：语言选择（JS/Python）、脚本代码
- **输入**：上游节点数据
- **输出**：转换后的数据
- **典型场景**：JSON 格式化、字段提取、正则匹配、数据聚合

## 边类型

### Flow (有向边)

- **样式**：实线 + 箭头（A → B）
- **用途**：执行流程控制 + 数据管道
- **数据传递**：上游节点的输出自动作为下游节点的输入
- **引用语法**：`{上游节点ID.output}` 或 `{节点ID.result.field}`
- **保留原有行为**：执行顺序、条件分支

### Data (无向边)

- **样式**：虚线 + 无箭头（A — B）
- **用途**：工具注册，仅 Agent 节点可发起
- **行为**：运行时 Agent 自动将连接的节点作为工具暴露给 LLM
- **限制**：只有 Agent 类型节点可以建立无向边
- **工具调用**：Agent 在 ReAct 循环中自动调用这些工具

### 边编辑器

- 点击边 → 弹出选择器 → 切换 Flow / Data 类型
- 创建边时默认为 Flow 类型
- 非 Agent 节点不允许创建 Data 边

## 数据管道机制

### 数据流

```
节点 A (产出) → 节点 B (消费) → 节点 C (消费)
```

- 节点 B 的输入 = 节点 A 的输出（自动注入）
- 节点配置中可用 `{节点ID.output}` 引用任意节点的输出
- 支持嵌套引用：`{节点ID.result.field.subfield}`

### 引用语法

在节点的配置字段中（如 API Call 的 Body、Condition 的表达式、Code 的脚本）：

```
{weather_data.output.temperature}
{agent_1.output.content}
{transform_1.result.processed}
```

### 状态管理

- 全局 `state` 保留，但节点间显式数据流优先
- 每个节点的输出存入 `state[节点ID]`
- 下游节点优先从上游节点引用数据，fallback 到全局 state

## 交互变更

### 节点样式

- 保持当前设计的左侧图标 + ID + 右侧状态圆圈
- 3px 彩色边框区分类型

### 边样式

- Flow（有向）：当前实线 + 箭头，颜色 `#94a3b8`
- Data（无向）：虚线 + 无箭头，颜色 `#8b5cf6`（紫色）

### 边选择器

- 点击边 → 弹出选择器
- 选项：Flow（默认）/ Data
- 数据导出时包含边类型字段

## 后端变更

### 边模型

```go
type EdgeType string
const (
    EdgeTypeFlow EdgeType = "flow"
    EdgeTypeData EdgeType = "data"
)

type Edge struct {
    From       string   `json:"from"`
    To         string   `json:"to"`
    EdgeType   EdgeType `json:"edge_type,omitempty"` // 默认 flow
    OutputPort string   `json:"output_port,omitempty"`
}
```

### 图构建

- Flow 边：保留现有构建逻辑，添加数据管道注入
- Data 边：不参与图构建，仅在 Agent 节点初始化时采集工具列表

### Agent 工具发现

- Agent 初始化时，扫描所有以 Data 边连接的目标节点
- 自动创建对应的 Tool 包装器
- 工具描述 = 节点描述，工具名称 = 节点 ID

## 数据流执行

### 数据管道注入逻辑

```
1. 节点 A 执行完毕，输出存入 state[A]
2. 节点 B 启动前，解析 B 配置中的 {A.output} 引用
3. 将解析后的数据注入 B 的输入
4. 节点 B 执行
```

### 引用解析器

- 解析 `{节点ID.output}` 格式
- 支持多级路径：`{节点ID.result.field}`
- 在节点执行前预处理，替换所有引用为实际值

## 前端变更

### 节点类型图标

- API Call：HTTP 请求 SVG 图标
- AI Agent：大脑/星形 SVG 图标
- Condition：菱形 SVG 图标
- Code/Transform：代码括号 SVG 图标

### 边管理

- 边对象新增 `edge_type` 字段
- 渲染时根据类型选择样式
- 点击边弹出选择器 UI
- 限制非 Agent 节点创建 Data 边

### 节点编辑器

- 新增节点类型选择：API Call / AI Agent / Condition / Code/Transform
- 移除 Approval（人工审批）相关代码
- 保留 LLM 配置但改名为 AI Agent 配置

## 实施计划

### Phase 1: 后端模型和数据层

1. 更新 `model/types.go`：Edge 结构体新增 EdgeType 字段
2. 更新 `engine/buildGraph`：处理 Data 边（跳过图构建）
3. 实现 Agent 工具发现机制（扫描 Data 边）
4. 实现数据管道引用解析器

### Phase 2: 前端节点和边

1. 更新节点类型列表（移除 Approval，新增 Code/Transform）
2. 实现双类型边渲染（Flow 实线 + 箭头，Data 虚线 + 无箭头）
3. 实现边选择器 UI（点击边切换类型）
4. 限制非 Agent 节点创建 Data 边

### Phase 3: 数据管道

1. 实现引用解析语法 `{节点ID.output}`
2. 数据管道注入逻辑
3. 更新节点配置面板支持引用

### Phase 4: Code/Transform 节点

1. 实现 JS 沙箱执行
2. 实现 Python 沙箱执行
3. 前端 Code/Transform 配置面板

## 移除项

- Approval（人工审批）节点类型
- Human Task 相关 API（后续可精简）
- 实例/任务页面中的审批相关 UI