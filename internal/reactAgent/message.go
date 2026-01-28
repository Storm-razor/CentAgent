package reactAgent

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

const systemPromptTemplate = `你是一名专业的 Docker 智能助手 CentAgent。
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

// MessageModifier 会在每次把所有历史消息传递给 ChatModel 之前执行
// 常用于添加前置的 system message
func MessageModify(ctx context.Context, input []*schema.Message) []*schema.Message {
	sanitized := input
	changed := false
	for i, m := range input {
		if m == nil {
			continue
		}
		if m.Role != schema.Assistant || len(m.ToolCalls) == 0 {
			continue
		}
		toolCallsChanged := false
		newToolCalls := m.ToolCalls
		for j := range m.ToolCalls {
			args := strings.TrimSpace(m.ToolCalls[j].Function.Arguments)
			if args == "" || args == "null" || !json.Valid([]byte(args)) {
				if !toolCallsChanged {
					newToolCalls = append([]schema.ToolCall(nil), m.ToolCalls...)
					toolCallsChanged = true
				}
				newToolCalls[j].Function.Arguments = "{}"
			}
		}
		if toolCallsChanged {
			if !changed {
				sanitized = append([]*schema.Message(nil), input...)
				changed = true
			}
			nm := *m
			nm.ToolCalls = newToolCalls
			sanitized[i] = &nm
		}
	}

	for _, m := range sanitized {
		if m == nil {
			continue
		}
		if m.Role == schema.System {
			return sanitized
		}
	}

	content := strings.NewReplacer(
		"{os}", runtime.GOOS,
		"{arch}", runtime.GOARCH,
		"{time}", time.Now().Format(time.RFC3339),
	).Replace(systemPromptTemplate)

	sys := schema.SystemMessage(content)
	out := make([]*schema.Message, 0, len(sanitized)+1)
	out = append(out, sys)
	out = append(out, sanitized...)
	return out
}

// MessageRewriter（消息重写器）会在调用 ChatModel 之前，修改存储在状态中的消息。
// 它接收累积在状态中的消息，对其进行修改，并将修改后的版本重新存入状态。
// 适用于压缩消息历史以适配模型上下文窗口，
// 或当您希望对在多次模型调用中都生效的消息进行修改时。
// 注意：如果同时设置了 MessageModifier 和 MessageRewriter，MessageRewriter 会在 MessageModifier 之前被调用。
func MessageRewrite(ctx context.Context, input []*schema.Message) []*schema.Message {
	//先不用
	return nil
}
