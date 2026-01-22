package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/wwwzy/CentAgent/internal/docker"
	"github.com/wwwzy/CentAgent/internal/monitor"
	"github.com/wwwzy/CentAgent/internal/storage"

	"github.com/spf13/cobra"
)

// startCmd 代表 start 命令
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动 CentAgent 监控服务",
	Long: `启动 CentAgent 后台监控服务。
这将初始化数据库，连接到 Docker，并开始收集统计信息和日志。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. 上下文用于优雅退出
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 2. 初始化存储
		fmt.Println("正在初始化存储...")
		store, err := storage.Open(ctx, cfg.Storage)
		if err != nil {
			return fmt.Errorf("打开存储失败: %w", err)
		}

		// 3. 检查 Docker 客户端
		fmt.Println("正在检查 Docker 连接...")
		if _, err := docker.GetClient(); err != nil {
			return fmt.Errorf("连接 docker 失败: %w", err)
		}

		// 4. 初始化监控管理器
		fmt.Println("正在初始化监控管理器...")
		mgr, err := monitor.NewManager(cfg.Monitor)
		if err != nil {
			return fmt.Errorf("创建监控管理器失败: %w", err)
		}

		// 5. 初始化并注入采集器
		stats, err := monitor.NewStatsCollector(store)
		if err != nil {
			return fmt.Errorf("创建 stats 采集器失败: %w", err)
		}

		logs, err := monitor.NewLogCollector(store)
		if err != nil {
			return fmt.Errorf("创建 log 采集器失败: %w", err)
		}

		ret, err := monitor.NewRetentionCollector(store)
		if err != nil {
			return fmt.Errorf("创建 retention 采集器失败: %w", err)
		}

		// 流式接口挂载采集器
		mgr.WithStats(stats).WithLogs(logs).WithRetention(ret)

		// 6. 启动管理器
		fmt.Println("正在启动监控服务...")
		if err := mgr.Start(ctx); err != nil {
			return fmt.Errorf("启动管理器失败: %w", err)
		}

		// 7. 等待信号
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		fmt.Println("CentAgent 已启动。按 Ctrl+C 停止。")

		select {
		case sig := <-sigChan:
			fmt.Printf("收到信号: %s, 正在关闭...\n", sig)
		case <-ctx.Done():
			fmt.Println("上下文已取消, 正在关闭...")
		}

		// 8. 优雅停止
		mgr.Stop()
		if err := mgr.Wait(); err != nil {
			return fmt.Errorf("管理器停止时发生错误: %w", err)
		}

		fmt.Println("关闭完成。")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	// 这里可以定义 start 命令特有的标志
	// startCmd.Flags().BoolP("daemon", "d", false, "以守护进程模式运行")
}
