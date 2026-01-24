# CentAgent 开发路线图 (Development Roadmap)

本文档基于 [Design.md](./Design.md) 设计规范与截至 2025-01-22 的代码实现状态，规划后续开发路径。

## 当前状态 (Current Status)

- **✅ 已完成 (Done)**:
  - **Monitor**: `internal/monitor` 实现容器状态 (Stats) 与日志 (Logs) 并发采集流水线，支持分层保留 (Retention) 策略。
  - **Docker**: `internal/docker` 完成 Docker SDK 的原子能力封装与测试。
  - **Storage**: `internal/storage` 完成 SQLite + GORM 持久化层与分层清理接口。
  - **Agent**: `internal/agent` 基于 Eino 完成 Agent 逻辑雏形 (Graph/Nodes)。

- **🚧 待完成 (Pending)**:
  - **CLI**: 缺少程序入口与命令解析 (`cmd/`, `internal/cli`)。
  - **TUI**: 缺少终端可视化交互界面 (`internal/tui`)。
  - **Integration**: Agent 与 Docker Tool 尚未在实际 CLI 中串联。

---

## 阶段规划 (Phased Plan)

### Phase 1: CLI 骨架与监控启动 (The "Ignition")
**目标**: 实现 `centagent start` 命令，打通 Monitor、Storage 和 Docker 的初始化链路，使后台监控真正运行起来。

1.  **构建入口**:
    - 创建 `cmd/centagent/main.go` 作为二进制入口。
    - 创建 `internal/cli/root.go` 定义 Cobra Root Command。
2.  **实现 Start 命令**:
    - 开发 `internal/cli/start.go`。
    - **初始化**: 负责 SQLite (`storage.New`) 与 Docker Client (`docker.NewClient`) 的生命周期管理。
    - **启动**: 组装并启动 `monitor.Manager` (默认开启 Stats, Logs, Retention)。
    - **退出**: 监听系统信号 (SIGINT/SIGTERM)，调用 `Manager.Stop()` 实现优雅退出。
3.  **验证**:
    - 运行 `go run ./cmd/centagent start`。
    - 观察 `centagent.db` 数据增长，验证监控流水线闭环。

### Phase 2: Agent 交互与工具链 (The "Brain")
**目标**: 实现 `centagent chat` 的后端逻辑，打通“用户 -> Agent -> Tool -> Docker”链路。

1.  **完善 Agent**:
    - 审查 `internal/agent`，确保 Tool 定义 (`internal/agent/tools.go`) 正确调用 `internal/docker` 的原子函数。
2.  **实现 Chat 命令**:
    - 开发 `internal/cli/chat.go`。
    - 在 CLI 中初始化 Eino Agent Graph。
    - 实现基础的控制台 REPL (Read-Eval-Print Loop) 用于调试对话逻辑。



### Phase 3: TUI 终端界面 (The "Dashboard")
**目标**: 引入 `Bubbletea` 实现设计文档中的现代化终端交互体验。

1.  **TUI 框架**:
    - 创建 `internal/ui`（UI 抽象接口）与 `internal/tui`（Bubbletea 实现）包，确保 UI 可插拔。
2.  **Chat UI**:
    - 实现“思考中”状态动画 (Thinking...)。
    - 实现 Markdown 消息流式渲染。
    - 实现 Tool 调用前的“用户决策确认”组件 (Yes/No 表单)。
    - 使用对话框UI分离AI与用户信息
    - 实现用户输入框与发送按钮

### Phase 4: 工程化增强 
1. **审计/可追溯**：
   - 将每次 tool 调用（名称、参数、结果摘要、耗时、是否被用户确认）写入 storage 的 Audit（库里已有 Audit 相关接口但尚未接线，见 repository.go ）。
2.  **“从 DB 查监控数据”的工具**：
   - Phase 2 先用 Docker tools 闭环，后续再把 QueryContainerStats/Logs 暴露为 tool，形成“实时 Docker + 历史 DB”的组合能力（更贴近 Agent 问答）

---

