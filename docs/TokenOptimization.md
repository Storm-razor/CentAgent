# CentAgent：降低上下文 Token 消耗的两种优化思路

本文档聚焦两类短期内收益明显、且与当前代码结构兼容的优化：

1. **(3) Tool 输出降噪**：工具输出通常是 JSON/日志长文本，重复塞进历史会迅速爆 token。
2. **(2) 按预算截断**：基于“预算”(字符数/估算 token)对历史进行裁剪，保证每轮输入稳定落在可控范围内。

> 适用现状：当前对话上下文主要依赖 `AgentState.Messages` 作为 history，并通过模板把 history 整体注入到模型输入中（参见 internal/agent 的 ChatModelNode 与 ToolsNode wrapper）。

---

## 背景：为什么 token 消耗会很快

当前链路的核心行为是：

- LLM 每轮会看到 `history = state.Messages`
- 工具执行的输出（`Role=tool`）会被追加回 `state.Messages`
- 下一轮又把这些 tool 输出原封不动带回去

当工具输出是：
- 容器 inspect 的完整 JSON
- 容器 logs 的大段文本
- list_containers 的大列表

这些内容一旦进了 history，之后每轮都会反复发送给模型，导致 token 成本线性甚至“指数式”增长（因为模型还会基于长上下文生成更长输出）。

---

## (3) Tool 输出降噪（推荐优先做）

### 目标

把 **“工具输出全文”** 从对话 history 中移除，改为：

- **全文落地到外部存储**（内存/文件/DB 任一皆可）
- **history 中只保留短摘要 + 引用(ref)**，模型需要细节时再通过工具按需拉取

### 核心原则

- history 只承载“决策与结论”，不承载“可再获取的大块数据”
- tool 输出是可再获取的（同一容器再取一次 logs/inspect），因此天然适合外置

### 推荐的数据形态（ref）

生成一个引用 ID（例如 UUID/自增序号），在 history 里写入类似：

- `工具 get_container_logs 执行完成：ref=toolout_123，行数=200，截断=否`
- `工具 inspect_container 执行完成：ref=toolout_124，字段数=...，原始 JSON 已保存`

#### 外部存储可选方案（按实现成本由低到高）

1. **进程内 LRU Map（最快落地）**
   - key: ref
   - value: 完整输出字符串 + 元数据（工具名、时间、容器 id）
   - 缺点：进程重启丢失

2. **落盘文件（简单可持久化）**
   - `./data/tool_outputs/{ref}.txt|.json`
   - metadata 可以写入旁路 index（jsonl）或内嵌在文件头

3. **落库（更工程化，后续可审计/检索）**
   - 新增表或复用现有审计结构（若已有）
   - 本文档只做思路，不要求现在实现

### 对模型侧的配套：提供“按需取回”工具

为了让模型能在“需要细节时”自己取回数据，建议新增 1 个工具：

- `get_tool_output(ref, start?, end?, grep?, json_path?)`

最小形态只支持 `ref` + `start/end`（按行截取）就足够处理 logs 场景。

### 你现有代码中推荐的插入点

1. **工具输出回写到 history 的地方**（最关键）
   - internal/agent 的 ToolsNode wrapper 会把 `outputs []*schema.Message` 直接 append 到 `state.Messages`
   - 优化点：在 append 前把 `msg.Content` 改成摘要 + ref，把全文写到外部存储

2. **工具定义返回值的地方（次选）**
   - internal/agent/tools.go 中每个 tool 的 `InvokableRun` 返回 string
   - 也可以在这里就进行“落地 + 返回 ref 摘要”，但这样会把“输出策略”分散到每个 tool，不利于统一管理

### 摘要策略（实用且简单）

建议根据工具名做轻量摘要，避免 JSON 直接进 history：

- list_containers：只保留 container_id/name/status 的前 N 条，附上“共 M 条”
- inspect_container：只保留 Name/State/Status/Ports/Image 等关键信息
- get_container_logs：只保留最后 N 行（比如 50 行）或只保留“行数 + ref”

### 风险与权衡

- 优点：token 降幅最大，且对长日志/大 JSON 非常有效
- 缺点：需要一个“外部存储 + ref”组件；模型想看细节需要再走一次工具调用

### 验证建议

- 连续对话 20 轮，观察：
  - 每轮请求体是否稳定（history 不再膨胀）
  - 模型在需要细节时是否能正确通过 ref 拉取

---

## (2) 按预算截断（稳定上限，适合作为兜底）

### 目标

不管用户聊多久，都保证“送给模型的上下文”不会超过一个固定预算。

### 预算的定义

按实现难度由低到高：

1. **字符数预算**（推荐先做）
   - 例如 history 的总字符数 <= 12000
   - 简单直接，跨模型也可用

2. **粗略 token 预算（估算）**
   - 例如：英文约 4 chars ≈ 1 token；中文约 1~2 chars ≈ 1 token（非常粗略）
   - 可以用启发式系数：`estimatedTokens = len(text)/2`（中文环境更保守）

3. **精确 token 预算**
   - 依赖具体模型 tokenizer，通常需要额外库或服务
   - 本文档不建议作为第一步

### 截断策略（建议组合）

优先级：先删“低价值且高体积”的消息，再删“高价值低体积”的消息。

常见顺序：

1. **先删 Role=tool 的长输出**（如果你没做(3)，这是救命稻草）
2. 再删最早的 assistant/user 对话
3. 仍超预算则做“早期对话摘要”替换（这部分属于增强，可后做）

### “预算截断”落点建议

落点应当在“真正喂给模型之前”，避免：
- state.Messages 虽然很长，但你每轮真正传给模型的是一个被裁剪后的列表

与你当前结构匹配的做法：

- 在 `ChatModelNode` 内部，使用 `state.Messages` 生成 `inputVars["history"]` 前，
  先构造一个 `historyForLLM := trimByBudget(state.Messages, budget)`
  然后 `inputVars["history"] = historyForLLM`

这样：
- `AgentState.Messages` 仍可保留“完整会话记录”（如果你需要）
- 但模型侧只看到“裁剪后的窗口”

如果你也不需要完整会话记录，则可以把裁剪直接作用到 `state.Messages` 本身（更省内存）。

### budget 选择建议

建议让 budget 可配置（CLI 或 config.yaml），但先给默认值：

- 字符预算（history 部分）：`12k ~ 20k chars`
- 如果工具输出没有降噪，预算要更小且更频繁触发截断，否则一次 tool 输出就能塞爆

### 风险与权衡

- 优点：实现简单，能提供“硬上限”，避免请求失败或极慢
- 缺点：可能裁掉模型需要的关键信息，尤其当对话跨主题、依赖早期约束时

---

## 两者如何搭配（推荐组合）

建议顺序：

1. **先做 (3) tool 输出降噪**：从源头减少最肥的上下文块
2. **再做 (2) 按预算截断**：作为兜底，保证任何情况下不爆

这样能同时做到：
- token 成本更低（降噪）
- token 上限可控（预算）

---

## 与当前 Phase 2 代码的结合建议（定位用）

可参考的核心位置：

- 工具输出追加到 history：internal/agent 的 ToolsNode wrapper（把 tool outputs append 回 state.Messages）
- 喂给模型 history 的入口：internal/agent 的 ChatModelNode（把 `state.Messages` 注入模板变量 `history`）
- CLI 侧只负责会话循环与确认交互：internal/cli/chat.go

后续你要做优化时，可以先从这两条线切入：

- 在 Tools 回写处“降噪/存 ref”
- 在 ChatModelNode 构造 history 时“按预算裁剪”

