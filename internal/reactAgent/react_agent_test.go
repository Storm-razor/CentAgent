package reactAgent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	agentflow "github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/schema"
	cb "github.com/cloudwego/eino/utils/callbacks"
	"github.com/stretchr/testify/require"
)

func formatMsg(m *schema.Message) string {
	if m == nil {
		return "<nil>"
	}
	var b strings.Builder
	b.WriteString(string(m.Role))
	if m.ToolName != "" {
		b.WriteString(" tool=")
		b.WriteString(m.ToolName)
	}
	if m.ToolCallID != "" {
		b.WriteString(" tool_call_id=")
		b.WriteString(m.ToolCallID)
	}
	if len(m.ToolCalls) > 0 {
		b.WriteString(" tool_calls=[")
		for i, tc := range m.ToolCalls {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(tc.Function.Name)
			if s := strings.TrimSpace(tc.Function.Arguments); s != "" {
				b.WriteString("(")
				if len(s) > 200 {
					s = s[:200] + "..."
				}
				b.WriteString(s)
				b.WriteString(")")
			}
		}
		b.WriteString("]")
	}
	content := strings.TrimSpace(m.Content)
	if content != "" {
		if len(content) > 400 {
			content = content[:400] + "..."
		}
		b.WriteString(" content=")
		b.WriteString(content)
	}
	return b.String()
}

func TestReActAgent_RealModel_PrintAllMessages(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("ARK_API_KEY"))
	modelID := strings.TrimSpace(os.Getenv("ARK_MODEL_ID"))
	base_url := strings.TrimSpace(os.Getenv("ARK_BASE_URL"))
	if apiKey == "" || modelID == "" {
		t.Skip("ARK_API_KEY 或 ARK_MODEL_ID 未设置，跳过真实 ReAct Agent 测试")
	}

	ctx := context.Background()

	agent, err := BuildAgent(ctx, ArkConfig{
		APIKey:  apiKey,
		ModelID: modelID,
		BaseURL: base_url,
	}, nil)
	require.NoError(t, err)

	var (
		mu       sync.Mutex
		captured []*schema.Message
	)
	addCaptured := func(ms ...*schema.Message) {
		mu.Lock()
		defer mu.Unlock()
		for _, m := range ms {
			if m == nil {
				continue
			}
			captured = append(captured, m)
		}
	}

	handler := cb.NewHandlerHelper().
		ChatModel(&cb.ModelCallbackHandler{
			OnEnd: func(ctx context.Context, _ *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
				if output != nil && output.Message != nil {
					addCaptured(output.Message)
				}
				return ctx
			},
		}).
		ToolsNode(&cb.ToolsNodeCallbackHandlers{
			OnEnd: func(ctx context.Context, _ *callbacks.RunInfo, output []*schema.Message) context.Context {
				addCaptured(output...)
				return ctx
			},
		}).
		Handler()

	history := MessageModify(ctx, []*schema.Message{
		schema.UserMessage("请列出当前运行的容器"),
	})

	out, err := agent.Generate(ctx, history, agentflow.WithComposeOptions(compose.WithCallbacks(handler)))
	require.NoError(t, err)
	require.NotNil(t, out)

	all := make([]*schema.Message, 0, len(history)+len(captured)+1)
	all = append(all, history...)
	all = append(all, captured...)
	all = append(all, out)

	for i, m := range all {
		fmt.Printf("[%02d] %s\n", i, formatMsg(m))
	}
}
