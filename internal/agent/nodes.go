package agent

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ChatModelNode 是 Graph 中的核心节点，负责：
// 1. 准备 ChatTemplate 所需的变量 (History, OS, Time 等)
// 2. 使用 ChatTemplate 生成 Messages
// 3. 调用 ChatModel 获取回复
// 4. 更新 AgentState (追加 AI Message, 填充 ToolCalls)
func ChatModelNode(ctx context.Context, state AgentState, chatModel model.ToolCallingChatModel) (AgentState, error) {
	// 1. 准备模板变量
	// 这里的 key 必须与 NewChatTemplate 中的 MessagesPlaceholder 和变量名一致
	// {os}, {arch}, {time} 是 SystemPromptTemplate 中的变量
	// {history} 是 NewChatTemplate 中的 MessagesPlaceholder
	inputVars := map[string]any{
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
		"time":    time.Now().Format(time.RFC3339),
		"history": state.Messages,
	}

	// 2. 生成消息列表
	// 使用 NewChatTemplate() 获取模板实例
	// 注意：为了性能，ChatTemplate 实例最好在 BuildGraph 时创建一次并复用，
	// 这里为了代码结构清晰，先在函数内创建，后续可优化为闭包传入
	template := NewChatTemplate()
	messages, err := template.Format(ctx, inputVars)
	if err != nil {
		return state, fmt.Errorf("format chat template failed: %w", err)
	}

	// 3. 调用 ChatModel
	// 这里使用 Generate 而不是 Stream，因为我们需要完整的 ToolCalls 信息来做路由决策
	// 如果需要流式输出给用户，可以在 OutputNode 中处理，或者使用 Stream 接口但在此处聚合
	aiMsg, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return state, fmt.Errorf("chat model generate failed: %w", err)
	}

	// 4. 更新状态
	// 追加 AI 回复到历史记录
	state.Messages = append(state.Messages, aiMsg)

	// 填充 NextStepToolCalls 供 Graph 分支判断
	state.NextStepToolCalls = aiMsg.ToolCalls

	// 清空上一轮的 ToolOutputs (因为已经处理完了)
	// 注意：ToolOutputs 已经在上一轮 Loop back 前被追加到了 Messages 中 (在 ToolsNode 中处理)
	// 所以这里不需要再处理 ToolOutputs，只需要清理信号字段
	state.LatestToolOutputs = nil

	return state, nil
}

// InputNode 处理用户输入，构建初始状态
func InputNode(ctx context.Context, state AgentState) (AgentState, error) {
	// 1. 如果 Messages 为空，说明是对话开始，注入 System Prompt
	// 注意：现在 System Prompt 由 ChatTemplate 统一管理，这里不再需要手动 append SystemMessage
	// 但为了兼容某些不使用 ChatTemplate 的场景，或者为了在 Messages 中保留完整的 Session 记录，
	// 我们可以选择在这里初始化一个空的 Messages slice
	if state.Messages == nil {
		state.Messages = make([]*schema.Message, 0)
	}

	// 2. 将 UserQuery 转换为 UserMessage 并追加
	// 注意：调用方可能已经把 UserQuery 放入了 Messages，这里做个检查
	// 如果 Messages 最后一个不是 User 且 UserQuery 不为空，则追加
	if state.UserQuery != "" {
		isLastUser := false
		if len(state.Messages) > 0 {
			lastMsg := state.Messages[len(state.Messages)-1]
			if lastMsg.Role == schema.User && lastMsg.Content == state.UserQuery {
				isLastUser = true
			}
		}

		if !isLastUser {
			userMsg := schema.UserMessage(state.UserQuery)
			state.Messages = append(state.Messages, userMsg)
		}
	}

	// 3. 清理上一轮的临时状态
	state.NextStepToolCalls = nil
	state.LatestToolOutputs = nil

	return state, nil
}
