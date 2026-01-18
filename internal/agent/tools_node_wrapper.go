package agent

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// NewToolsNode 创建一个符合 Eino compose.ToolsNode 规范的节点
func NewToolsNode(ctx context.Context, config *compose.ToolsNodeConfig) (*compose.ToolsNode, error) {
	// 创建 Eino 原生 ToolsNode
	tn, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: config.Tools,
	})
	return tn, err
}

// ConvertStateToToolsInput 将 AgentState 转换为 ToolsNode 所需的输入 (*schema.Message)
func ConvertStateToToolsInput(ctx context.Context, state AgentState) (*schema.Message, error) {
	// 构造一个包含 ToolCalls 的 Message 作为输入
	// ToolsNode 会解析这个 Message 中的 ToolCalls 并执行
	return &schema.Message{
		Role:      schema.Assistant,
		ToolCalls: state.NextStepToolCalls,
	}, nil
}

// ConvertToolsOutputToState 将 ToolsNode 的输出 ([]*schema.Message) 转换回 AgentState
func ConvertToolsOutputToState(ctx context.Context, state AgentState, outputs []*schema.Message) (AgentState, error) {
	// 1. 更新 LatestToolOutputs
	state.LatestToolOutputs = outputs

	// 2. 将 ToolOutputs 追加到 Messages
	state.Messages = append(state.Messages, outputs...)

	// 3. 清空 NextStepToolCalls
	state.NextStepToolCalls = nil

	return state, nil
}
