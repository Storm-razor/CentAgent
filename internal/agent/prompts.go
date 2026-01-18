package agent

import (
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// SystemPromptTemplate 定义系统提示词模板
// 包含动态变量: {time}, {os}, {arch}
const SystemPromptTemplate = `你是一名专业的 Docker 智能助手 CentAgent。
你的目标是帮助用户管理、监控和诊断 Docker 容器问题。

当前运行环境:
- 操作系统: {os}
- 架构: {arch}
- 系统时间: {time}

你需要遵循以下原则:
1. 在执行删除、停止等高风险操作前，必须明确告知用户风险。
2. 如果用户询问日志，优先查看最近的异常日志。
3. 回答要简洁明了，命令输出如果过长，请进行摘要。
4. 如果遇到无法解决的问题，建议用户查阅官方文档。

你可以使用的工具包括 Docker 容器管理、镜像管理、网络管理等。
请根据用户的输入，选择合适的工具或直接回答。`

// NewChatTemplate 创建一个 ChatTemplate 实例
// 该模板用于将 AgentState 中的数据转换为 ChatModel 可接受的消息列表
func NewChatTemplate() prompt.ChatTemplate {
	return prompt.FromMessages(schema.FString,
		// 1. 系统消息 (包含动态环境信息)
		schema.SystemMessage(SystemPromptTemplate),

		// 2. 历史消息占位符 (用于注入对话历史)
		// "history" 是参数名，true 表示该字段是可选的
		schema.MessagesPlaceholder("history", true),

		// 3. 用户当前输入 (可选，如果 history 中已经包含了最新用户消息，这里可以省略，
		// 但为了灵活性，我们保留这个占位符，由调用方决定是否传入 "query")
		// 这里我们假设 history 包含了之前的对话，而 query 是本轮最新的输入
		// 如果 InputNode 已经把 UserQuery append 到了 history，则这里不需要 UserMessage
		// 这里的实现策略是：ChatTemplate 负责组装 "System + History + (Optional Query)"
		// 为了防止重复，我们在 Graph 逻辑中控制：如果 Messages 里最后一条不是 UserQuery，则传入 query
		// 暂时我们只定义结构，具体传入什么由 Node 决定
	)
}
