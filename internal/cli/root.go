package cli

import (
	"fmt"
	"os"

	"github.com/wwwzy/CentAgent/internal/config"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	cfg     *config.Config
)

// rootCmd 是没有子命令时调用的基础命令
var rootCmd = &cobra.Command{
	Use:   "centagent",
	Short: "CentAgent 是一个 Docker 监控和 AI 代理",
	Long: `CentAgent 监控 Docker 容器并提供 AI 驱动的接口
来与您的容器化环境进行交互和管理。`,
	// 如果您的纯应用程序有关联的操作，请取消注释以下行：
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute 将所有子命令添加到根命令并适当设置标志。
// 这由 main.main() 调用。它只需要对 rootCmd 调用一次。
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// 在这里定义您的标志和配置设置。
	// Cobra 支持持久标志，如果在定义在这里，
	// 将对您的应用程序全局有效。
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件（默认按 ./config.yaml、./configs/config.yaml、$HOME/.centagent/config.yaml 搜索）")
}

// initConfig 读取配置文件和环境变量（如果已设置）。
func initConfig() {
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}
}
