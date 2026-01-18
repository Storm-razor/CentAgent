# Eino Graph Orchestration Design

基于 CloudWeGo Eino 框架的 Graph 编排设计。本设计旨在实现一个支持 Tool Calling、状态保持和用户交互 (Human-in-the-loop) 的 ReAct 智能体。

## 1. 核心概念 (Core Concepts)

Eino 强调 **类型安全 (Type Safety)** 和 **流式处理 (Stream Awareness)**。我们的 Graph 将由以下核心节点 (Nodes) 和 边 (Edges) 组成。

### 数据状态定义 (State Schema)

在 Graph 中流转的全局状态对象 `AgentState`：

```go
type AgentState struct {
    // 历史对话消息 (包含 User, System, AI, Tool 消息)
    Messages []schema.Message 
    
    // 当前上下文信息 (如当前选中的容器 ID)
    Context  map[string]interface{}
    
    // 待执行的工具调用 (LLM 生成)
    ToolCalls []schema.ToolCall
    
    // 工具执行结果
    ToolOutputs []schema.ToolOutput
    
    // 用户最后的指令 (用于重试或澄清)
    UserQuery string
}
```

---

## 2. Graph 拓扑结构 (Topology)

我们将构建一个有环图 (Cyclic Graph) 来实现 ReAct 循环。

```mermaid
graph TD
    Start((Start)) --> InputNode
    InputNode[Input Processing] --> ChatModelNode
    
    ChatModelNode[Chat Model (LLM)] -->|Has Tool Calls| ToolNode
    ChatModelNode -->|No Tool Calls| OutputNode
    
    ToolNode[Tool Executor] -->|Update History| ChatModelNode
    
    OutputNode[Response Generator] --> End((End))
```

### 节点详细设计 (Node Details)

#### 1. InputNode (预处理)
*   **Input**: `User Input String`
*   **Output**: `AgentState`
*   **Logic**: 
    *   加载历史对话 (Memory Load)。
    *   注入 System Prompt (包含 Docker 管理员角色设定、当前系统时间等)。
    *   构建初始 `AgentState`。

#### 2. ChatModelNode (核心推理)
*   **Input**: `AgentState`
*   **Output**: `AgentState` (Updated with AI Message)
*   **Logic**:
    *   调用 LLM (如 DeepSeek/OpenAI)。
    *   绑定 Tools 定义 (Docker Tools)。
    *   **关键**: LLM 会返回自然语言回复，或者 `ToolCalls` 指令。
    *   更新 `AgentState.Messages` 追加 AI 的回复。

#### 3. ToolNode (工具执行)
*   **Input**: `AgentState` (Containing ToolCalls)
*   **Output**: `AgentState` (Updated with ToolOutputs)
*   **Logic**:
    *   解析 `ToolCalls`。
    *   **Human-in-the-loop (交互介入)**:
        *   对于高危操作 (如 `container_stop`, `container_remove`)，在此处触发 TUI 弹窗确认。
        *   若用户拒绝，生成 "User rejected operation" 的 ToolOutput。
        *   若用户同意，调用 Docker SDK 执行。
    *   将执行结果封装为 `ToolOutput` 消息。
    *   追加到 `AgentState.Messages`。

#### 4. OutputNode (响应生成)
*   **Input**: `AgentState`
*   **Output**: `Stream<String>` / `String`
*   **Logic**:
    *   负责将最终的 AI 回复格式化输出给用户。
    *   保存新的对话记录到 Memory (Memory Save)。

---

## 3. 边与条件 (Edges & Branches)

使用 Eino 的 `Graph.AddBranch` 实现条件跳转：

1.  **`should_continue` (ChatModelNode -> ToolNode/OutputNode)**
    *   **Condition**: 检查 `LastMessage.ToolCalls` 是否非空。
    *   **True**: 跳转至 `ToolNode`。
    *   **False**: 跳转至 `OutputNode`。

2.  **`tool_feedback` (ToolNode -> ChatModelNode)**
    *   **Always**: 执行完工具后，必须跳回 LLM，让 LLM 根据工具结果生成最终回复（或进行下一步操作）。

---

## 4. Eino 代码实现规划 (Implementation Plan)

### 阶段 1: 定义组件 (Components)
在 `internal/agent/components` 下定义：
*   `NewChatModel()`: 初始化 LLM 模型配置。
*   `NewToolsNode()`: 封装 Docker Tools。

### 阶段 2: 编排 Graph (Orchestration)
在 `internal/agent/graph.go` 中：

```go
// 伪代码示例
func BuildGraph() (eino.Runnable, error) {
    g := compose.NewGraph[AgentState, AgentState]()
    
    g.AddNode("chat_model", chatModel)
    g.AddNode("tools", toolsNode)
    
    // 边与分支
    g.AddEdge(compose.START, "chat_model")
    g.AddBranch("chat_model", func(s AgentState) string {
        if len(s.LastMessage.ToolCalls) > 0 {
            return "tools"
        }
        return "end"
    }, map[string]string{
        "tools": "tools",
        "end":   compose.END,
    })
    g.AddEdge("tools", "chat_model") // Loop back
    
    return g.Compile()
}
```

### 阶段 3: 绑定 TUI
在 `ToolNode` 的具体实现中，注入 `TUIController` 接口，用于在执行具体 Tool 前调用 `tui.Confirm()`。

---


