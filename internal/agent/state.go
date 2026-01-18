package agent

import (
	"github.com/cloudwego/eino/schema"
)

// AgentState 定义了在 Graph 中流转的状态
type AgentState struct {
	// 历史对话消息 (包含 User, System, AI, Tool 消息)
	Messages []*schema.Message `json:"messages"`

	// 当前上下文信息 (如当前选中的容器 ID, 系统环境等)
	Context map[string]interface{} `json:"context"`

	// 显式信号字段，用于 Graph 分支判断
	NextStepToolCalls []schema.ToolCall `json:"tool_calls"`   // 本轮 LLM 生成的工具调用
	LatestToolOutputs []*schema.Message `json:"tool_outputs"` // 本轮工具执行后的结果消息 (Role=Tool)

	// 用户最后的指令 (用于重试或澄清)
	UserQuery string `json:"user_query"`
}
