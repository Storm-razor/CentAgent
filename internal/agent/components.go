package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
)

// NewChatModel 初始化 Ark ChatModel
func NewChatModel(ctx context.Context) (*ark.ChatModel, error) {
	// 优先从环境变量读取配置，后续可改为从 config 包读取
	apiKey := os.Getenv("ARK_API_KEY")
	modelID := os.Getenv("ARK_MODEL_ID")
	baseURL := os.Getenv("ARK_BASE_URL")

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
