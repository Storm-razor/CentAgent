package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
)

const (
	NodeInput     = "input_node"
	NodeChatModel = "chat_model_node"
	NodeTools     = "tools_node"
)

type ArkConfig struct {
	APIKey  string `mapstructure:"api_key"`
	ModelID string `mapstructure:"model_id"`
	BaseURL string `mapstructure:"base_url"`
}

// BuildGraph 构建 Agent 的处理流程图
func BuildGraph(ctx context.Context, arkConfig ArkConfig) (compose.Runnable[AgentState, AgentState], error) {
	//获取chatModel
	cm, err := NewChatModel(ctx, arkConfig)
	if err != nil {
		return nil, fmt.Errorf("init chat model failed: %w", err)
	}

	// 初始化 Graph，输入输出都是 AgentState
	g := compose.NewGraph[AgentState, AgentState]()

	// 1. 添加节点
	// InputNode: 简单的透传或预处理
	g.AddLambdaNode(NodeInput, compose.InvokableLambda(InputNode))

	// ChatModelNode: 核心 LLM 推理节点
	// 使用闭包注入 chatModel
	g.AddLambdaNode(NodeChatModel, compose.InvokableLambda(func(ctx context.Context, state AgentState) (AgentState, error) {
		return ChatModelNode(ctx, state, cm)
	}))

	// ToolsNode: 工具执行节点
	// 创建 ToolsNode
	tools := GetTools()
	tn, err := NewToolsNode(ctx, &compose.ToolsNodeConfig{Tools: tools})
	if err != nil {
		return nil, fmt.Errorf("create tools node failed: %w", err)
	}

	// 将工具信息添加到chatModel
	toolsInfo, err := GetToolsInfo(ctx)
	err = cm.BindTools(toolsInfo)
	if err != nil {
		return nil, fmt.Errorf("bind tools to chat model failed: %w", err)
	}

	// 2. 添加到 Graph
	g.AddLambdaNode(NodeTools, compose.InvokableLambda(func(ctx context.Context, state AgentState) (AgentState, error) {
		// 转换输入
		inputMsg, err := ConvertStateToToolsInput(ctx, state)
		if err != nil {
			return state, err
		}

		// 调用 ToolsNode
		outputs, err := tn.Invoke(ctx, inputMsg)
		if err != nil {
			return state, err
		}

		// 转换输出
		return ConvertToolsOutputToState(ctx, state, outputs)
	}))

	// 2. 添加边 (Edges)
	// Start -> Input
	if err := g.AddEdge(compose.START, NodeInput); err != nil {
		return nil, err
	}

	// Input -> ChatModel
	if err := g.AddEdge(NodeInput, NodeChatModel); err != nil {
		return nil, err
	}

	// 3. 添加分支 (Branches)
	// ChatModel -> Tools OR End
	// 如果 LLM 返回了 ToolCalls，则去 ToolsNode，否则结束
	err = g.AddBranch(NodeChatModel, compose.NewGraphBranch(func(ctx context.Context, state AgentState) (string, error) {
		// 使用 State 中显式的 NextStepToolCalls 字段判断
		if len(state.NextStepToolCalls) > 0 {
			return NodeTools, nil
		}
		return compose.END, nil
	}, map[string]bool{
		NodeTools:   true,
		compose.END: true,
	}))
	if err != nil {
		return nil, err
	}

	// Tools -> ChatModel (Loop back)
	// 工具执行完后，将结果返回给 LLM 继续思考
	if err := g.AddEdge(NodeTools, NodeChatModel); err != nil {
		return nil, err
	}

	// 4. 编译 Graph
	runnable, err := g.Compile(ctx)
	if err != nil {
		return nil, err
	}

	return runnable, nil
}
