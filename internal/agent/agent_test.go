package agent

import (
	"context"
	"os"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// TestRealAgentGraphFlow 使用真实的 ChatModel 进行集成测试
// 该测试需要 ARK_API_KEY 和 ARK_MODEL_ID 环境变量
// 如果未设置，测试将跳过
func TestRealAgentGraphFlow(t *testing.T) {
	// 1. 检查环境变量
	apiKey := os.Getenv("ARK_API_KEY")
	modelID := os.Getenv("ARK_MODEL_ID")
	if apiKey == "" || modelID == "" {
		t.Skip("Skipping real agent test: ARK_API_KEY or ARK_MODEL_ID not set")
	}

	ctx := context.Background()

	// 2. 构建 Graph
	// 直接使用 BuildGraph，它会调用 NewChatModel 初始化真实的 Ark 模型
	runnable, err := BuildGraph(ctx)
	if err != nil {
		t.Fatalf("Failed to build graph: %v", err)
	}

	// 3. 运行 Graph
	// 测试场景：询问用户 Docker 版本（通常不需要复杂工具，或者可以用 info 工具）
	// 或者询问列出容器（这会触发 ListContainersTool）
	// 为了确保测试稳定性，我们询问一个需要调用 list_containers 的问题
	initialState := AgentState{
		UserQuery: "List all docker containers and tell me which ones are running",
	}

	t.Log("Starting agent execution with query:", initialState.UserQuery)
	finalState, err := runnable.Invoke(ctx, initialState)
	if err != nil {
		t.Fatalf("Graph execution failed: %v", err)
	}

	// 4. 验证结果
	msgs := finalState.Messages
	t.Logf("Total messages: %d", len(msgs))

	// 打印每一条消息以便调试
	for i, msg := range msgs {
		content := msg.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		t.Logf("[%d] Role=%s Content=%s ToolCalls=%v", i, msg.Role, content, msg.ToolCalls)
	}

	// 验证基本逻辑：
	// 1. 至少应该有 User -> AI (ToolCall) -> Tool -> AI (Final) 这样的流转
	// 所以消息数通常 >= 3 (System + User + AI + Tool + AI)
	if len(msgs) < 3 {
		t.Error("Expected multi-turn conversation (User -> AI -> Tool -> AI), but got too few messages")
	}

	lastMsg := msgs[len(msgs)-1]
	if lastMsg.Role != schema.Assistant {
		t.Errorf("Expected final message from Assistant, got %s", lastMsg.Role)
	}
	
	// 验证是否真的触发了 Tool
	hasToolCall := false
	hasToolOutput := false
	for _, msg := range msgs {
		if len(msg.ToolCalls) > 0 {
			hasToolCall = true
			for _, tc := range msg.ToolCalls {
				t.Logf("Tool Call Detected: %s(%s)", tc.Function.Name, tc.Function.Arguments)
				if tc.Function.Name != "list_containers" {
					t.Logf("Warning: Expected list_containers tool call, got %s", tc.Function.Name)
				}
			}
		}
		if msg.Role == schema.Tool {
			hasToolOutput = true
			t.Logf("Tool Output Detected: %s...", msg.Content[:min(len(msg.Content), 50)])
		}
	}

	if !hasToolCall {
		t.Error("Agent did not make any tool calls")
	}
	if !hasToolOutput {
		t.Error("No tool output found in history")
	}

	t.Logf("Final response: %s", lastMsg.Content)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
