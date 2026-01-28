package reactAgent

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/wwwzy/CentAgent/internal/storage"
)

type ArkConfig struct {
	APIKey  string `mapstructure:"api_key"`
	ModelID string `mapstructure:"model_id"`
	BaseURL string `mapstructure:"base_url"`
}

func BuildAgent(ctx context.Context, arkConfig ArkConfig, store *storage.Storage) (*react.Agent, error) {
	chatModel, err := NewChatModel(ctx, arkConfig)
	if err != nil {
		return nil, err
	}

	toolsInfo, err := GetToolsInfo(ctx, store)
	if err != nil {
		return nil, err
	}
	toolCallingModel, err := chatModel.WithTools(toolsInfo)
	if err != nil {
		return nil, err
	}

	tools := compose.ToolsNodeConfig{
		Tools: GetTools(store),
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: toolCallingModel,
		ToolsConfig:      tools,
		MessageModifier:  MessageModify,
		//MessageRewriter: MessageRewrite,
		MaxStep: 20,
	})
	if err != nil {
		return nil, err
	}
	return agent, nil
}
