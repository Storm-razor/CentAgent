package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/wwwzy/CentAgent/internal/agent"
	"github.com/wwwzy/CentAgent/internal/docker"
	"github.com/wwwzy/CentAgent/internal/tui"
	"github.com/wwwzy/CentAgent/internal/ui"
)

var chatConfirmTools bool
var chatUI string

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

		var uiImpl ui.ChatUI
		switch chatUI {
		case "console", "":
			uiImpl = &ui.ConsoleChatUI{In: os.Stdin, Out: os.Stdout}
		case "tui":
			uiImpl = &tui.ChatUI{}
		default:
			return fmt.Errorf("未知 ui 类型: %s (支持: console, tui)", chatUI)
		}

		return uiImpl.Run(ctx, runnable, ui.DefaultInitialState(), ui.ChatOptions{
			ConfirmTools: chatConfirmTools,
		})
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().BoolVar(&chatConfirmTools, "confirm-tools", true, "工具调用前询问确认")
	chatCmd.Flags().StringVar(&chatUI, "ui", "console", "交互界面类型: console/tui")
}
