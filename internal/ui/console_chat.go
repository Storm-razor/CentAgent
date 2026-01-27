package ui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/wwwzy/CentAgent/internal/agent"
)

type ConsoleChatUI struct {
	In  io.Reader
	Out io.Writer
}

func (u *ConsoleChatUI) Run(ctx context.Context, backend ChatBackend, initial agent.AgentState, opts ChatOptions) error {
	in := u.In
	if in == nil {
		return fmt.Errorf("console ui: In is nil")
	}
	out := u.Out
	if out == nil {
		return fmt.Errorf("console ui: Out is nil")
	}

	reader := bufio.NewReader(in)
	state := initial
	if state.Context == nil {
		state.Context = map[string]interface{}{}
	}

	fmt.Fprintln(out, "进入 CentAgent 对话模式。输入 exit/quit 退出。")
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(out, "已退出。")
			return nil
		default:
		}

		state.Context[agent.ConfirmEnabledContextKey] = opts.ConfirmTools

		if awaiting, ok := state.Context[agent.ConfirmAwaitingContextKey].(bool); ok && awaiting {
			fmt.Fprint(out, "确认执行工具？(y/N): ")
			line, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("读取输入失败: %w", err)
			}
			line = strings.TrimSpace(line)
			switch strings.ToLower(line) {
			case "exit", "quit":
				fmt.Fprintln(out, "已退出。")
				return nil
			}

			granted := strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")
			state.Context[agent.ConfirmGrantedContextKey] = granted
			if granted {
				state.UserQuery = ""
			} else {
				state.UserQuery = "我拒绝执行工具操作，请给出替代方案。"
			}
		} else {
			fmt.Fprint(out, "你: ")
			line, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("读取输入失败: %w", err)
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			switch strings.ToLower(line) {
			case "exit", "quit":
				fmt.Fprintln(out, "已退出。")
				return nil
			}
			state.UserQuery = line

			// 每次新用户查询生成一个 TraceID
			state.Context[agent.ConfirmEnabledContextKey] = opts.ConfirmTools
			traceID := uuid.New().String()
			// 将 TraceID 注入 context
			ctx = agent.WithTraceID(ctx, traceID)
			// 注意：agent state 的 Context 主要是用于 Eino 内部传递，
			// 而 tool 的 InvokableRun 接收的是 ctx 参数。
			// 为了保险起见，我们也可以在 agent state 的 context 里放一份，
			// 但 Eino graph 节点传递 context 的机制是主要的。
			// 不过 Eino 每次 Invoke 可能会重置 context，所以这里修改 ctx 是对的，
			// 只要 backend.Invoke 传递了这个 ctx。
		}

		var err error
		state, err = backend.Invoke(ctx, state)
		if err != nil {
			return err
		}

		if len(state.Messages) == 0 {
			fmt.Fprintln(out, "助手: (无输出)")
			fmt.Fprintln(out)
			continue
		}

		if printed := printLastAssistant(out, state.Messages); !printed {
			fmt.Fprintln(out, "助手: (无最终回复)")
		}
		fmt.Fprintln(out)
	}
}

func printLastAssistant(w io.Writer, messages []*schema.Message) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != schema.Assistant {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			fmt.Fprintln(w, "助手: (无文本输出)")
		} else {
			fmt.Fprintf(w, "助手: %s\n", content)
		}
		return true
	}
	return false
}
