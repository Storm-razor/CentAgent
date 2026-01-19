# CentAgent 项目进度分析与修复计划

根据文档设计与代码现状的对比分析，当前项目处于 **阶段 2：Agent 核心编排 (Orchestration)** 的中间状态。基础设施（Docker SDK）已就绪，Agent 骨架已搭建，但核心链路尚未打通，且存在阻断性 Bug。

## 1. 设计与实现对齐分析

| 模块 | 设计文档 (Design.md / EinoGraphDesign.md) | 代码现状 (internal/agent) | 状态 |
| :--- | :--- | :--- | :--- |
| **拓扑结构** | Start -> Input -> ChatModel <-> Tools -> Output | `graph.go` 实现了 Input/ChatModel/Tools 的循环结构。OutputNode 逻辑目前隐式包含在结束流程中。 | ✅ 基本符合 |
| **状态定义** | `AgentState` 含 Messages, Context, ToolCalls 等 | `state.go` 定义完全一致。 | ✅ 一致 |
| **Docker能力** | List, Inspect, Logs, Lifecycle | `internal/docker` 已实现基础函数，但**未封装为 Eino Tools**。 | ⚠️ 需封装 |
| **LLM 配置** | Ark / OpenAI | `components.go` 实现了 Ark，但**配置参数错误**。 | ❌ 需修复 |
| **工具绑定** | `BindTools` 动态绑定 | `graph.go` 中调用了 `BindTools` 但**参数和逻辑缺失**，导致无法编译。 | ❌ 阻塞中 |

## 2. 当前进度概览

- **已完成 (Done)**:
  - [x] Docker 基础操作库 (`internal/docker`)：支持容器列表、详情、日志、起停。
  - [x] Agent 状态定义 (`state.go`)。
  - [x] ReAct Graph 骨架代码 (`graph.go`, `nodes.go`)。
  - [x] Prompt 模板 (`prompts.go`)。

- **进行中 (In Progress) / 阻塞点**:
  - [ ] **Tools 封装**: 缺少 `GetTools()` 实现，无法将 Docker 函数暴露给 Agent。
  - [ ] **Graph 编译修复**: `graph.go` 因缺少 `GetTools` 和错误的 `BindTools` 调用而无法编译。
  - [ ] **ChatModel 修正**: 环境变量映射错误，会导致运行时连接失败。

- **待开始 (Pending)**:
  - [ ] CLI 入口 (`cmd/`, `internal/cli`)。
  - [ ] TUI 交互 (`internal/tui`)。
  - [ ] 数据存储与监控 (`internal/storage`, `internal/monitor`)。

## 3. 下一步修复与开发计划

为了让 Agent 核心能够运行（Run），我将按以下顺序执行修复：

1.  **修复 ChatModel 配置**: 修正 `components.go` 中的 Ark 环境变量映射。
2.  **封装 Docker Tools**:
    - 在 `internal/agent/tools.go` (新建) 中，使用 Eino `utils.InferTool` 将 `internal/docker` 的函数封装为 Tool。
    - 实现 `GetTools()` 函数。
3.  **修复 Graph 编排**:
    - 修正 `graph.go` 中的 `BindTools` 调用逻辑。
    - 完善 `BuildGraph` 的错误处理。
4.  **验证运行**: 创建一个简单的测试入口 `cmd/test_agent/main.go` 来验证 Agent 的问答循环。

确认无误后，我们将拥有一个可运行的、具备 Docker 管理能力的 Agent 核心。
