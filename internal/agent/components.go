package agent

import (
	"context"
	"os"
	"fmt"
	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/model"
)

// NewChatModel 初始化 Ark ChatModel
func NewChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	// 优先从环境变量读取配置，后续可改为从 config 包读取
	apiKey := os.Getenv("ARK_API_KEY")
	baseURL := os.Getenv("ARK_MODEL_NAME")
	modelName := os.Getenv("ARK_BASE_URL")

	if apiKey == "" || modelName == "" || baseURL== ""{
		// 默认 fallback 或者报错，这里暂且留空让 SDK 报错或使用默认
		return nil, fmt.Errorf("ARK_API_KEY, ARK_MODEL_NAME, ARK_BASE_URL must be set")
	}

	config := &ark.ChatModelConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   modelName,
	}

	chatModel, err := ark.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}

	return chatModel, nil
}
