package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/wwwzy/CentAgent/internal/config"
)

// NewChatModel 初始化 Ark ChatModel
func NewChatModel(ctx context.Context, arkConfig config.ArkConfig) (*ark.ChatModel, error) {
	// 优先从环境变量读取配置，后续可改为从 config 包读取
	apiKey := arkConfig.APIKey
	modelID := arkConfig.ModelID
	baseURL := arkConfig.BaseURL

	if apiKey == "" || modelID == "" {
		// 默认 fallback 或者报错，这里暂且留空让 SDK 报错或使用默认
		return nil, fmt.Errorf("ARK_API_KEY, ARK_MODEL_ID must be set")
	}

	config := &ark.ChatModelConfig{
		APIKey:  apiKey,
		Model:   modelID,
		BaseURL: baseURL,
	}

	chatModel, err := ark.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}

	return chatModel, nil
}
