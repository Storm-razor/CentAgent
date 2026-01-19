# CentAgent Agent 实现计划

根据您的要求，我将重点在 `internal/agent` 目录中实现 Docker 相关的 Tool，并修复阻碍 Agent 运行的关键配置和编排代码。

## 1. 核心任务：封装 Docker Tools
在 `internal/agent/tools.go` 中创建以下工具，每个工具都实现 `InvokableTool` 接口（`Info` 和 `InvokableRun` 方法）：

| 工具名称 | 对应 Docker 功能 | 参数定义 (JSON Schema) |
| :--- | :--- | :--- |
| `ListContainersTool` | `docker.ListContainers` | `all` (bool), `limit` (int), `status` (string) |
| `InspectContainerTool` | `docker.InspectContainer` | `container_id` (string) |
| `GetContainerLogsTool` | `docker.GetContainerLogs` | `container_id` (string), `tail` (string), `since` (string) |
| `StartContainerTool` | `docker.StartContainer` | `container_id` (string) |
| `StopContainerTool` | `docker.StopContainer` | `container_id` (string) |
| `RestartContainerTool` | `docker.RestartContainer` | `container_id` (string) |

同时导出 `GetTools() []tool.BaseTool` 函数供 Graph 使用。

## 2. 关联修复 (Fixes)
为了确保新实现的 Tools 能在 Agent 中正常工作，必须修复以下现有问题：

1.  **修复 `components.go`**: 修正 Ark ChatModel 的环境变量读取逻辑（Key, Model, BaseURL 映射错误）。
2.  **修复 `graph.go`**:
    - 引入 `GetTools()`。
    - 修正 `BindTools` 的调用方式（将工具绑定到模型）。
    - 修复 `BuildGraph` 未检查 `NewChatModel` 错误的问题。

## 3. 执行步骤
1.  创建 `internal/agent/tools.go` 并实现上述 6 个工具。
2.  修改 `internal/agent/components.go` 修正配置错误。
3.  修改 `internal/agent/graph.go` 正确集成 Tools 和 ChatModel。
