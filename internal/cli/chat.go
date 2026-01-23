package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/cobra"
	"github.com/wwwzy/CentAgent/internal/agent"
	"github.com/wwwzy/CentAgent/internal/docker"
)

var chatConfirmTools bool

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "进入交互式对话模式",
	Long: `进入一个简单的控制台 REPL，用自然语言管理 Docker 容器。
在必要时，Agent 会调用内置工具来获取信息或执行操作。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			cancel()
		}()

		if _, err := docker.GetClient(); err != nil {
			return fmt.Errorf("连接 docker 失败: %w", err)
		}

		runnable, err := agent.BuildGraph(ctx, cfg.Ark)
		if err != nil {
			return fmt.Errorf("构建 Agent Graph 失败: %w", err)
		}

		reader := bufio.NewReader(os.Stdin)
		state := agent.AgentState{
			Messages: make([]*schema.Message, 0),
			Context:  map[string]interface{}{},
		}

		fmt.Println("进入 CentAgent 对话模式。输入 exit/quit 退出。")
		for {
			select {
			case <-ctx.Done():
				fmt.Println("已退出。")
				return nil
			default:
			}

			state.Context[agent.ConfirmEnabledContextKey] = chatConfirmTools

			if awaiting, ok := state.Context[agent.ConfirmAwaitingContextKey].(bool); ok && awaiting {
				fmt.Print("确认执行工具？(y/N): ")
				line, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("读取输入失败: %w", err)
				}
				line = strings.TrimSpace(line)
				switch strings.ToLower(line) {
				case "exit", "quit":
					fmt.Println("已退出。")
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
				fmt.Print("你: ")
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
					fmt.Println("已退出。")
					return nil
				}
				state.UserQuery = line
			}

			state, err = runnable.Invoke(ctx, state)
			if err != nil {
				return err
			}

			if len(state.Messages) == 0 {
				fmt.Println("助手: (无输出)")
				continue
			}

			if printed := printLastAssistant(state.Messages); !printed {
				fmt.Println("助手: (无最终回复)")
			}
			fmt.Println()
		}
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().BoolVar(&chatConfirmTools, "confirm-tools", true, "工具调用前询问确认")
}

func printLastAssistant(messages []*schema.Message) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != schema.Assistant {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			fmt.Println("助手: (无文本输出)")
		} else {
			fmt.Printf("助手: %s\n", content)
		}
		return true
	}
	return false
}
