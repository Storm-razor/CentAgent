package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"time"

	"github.com/spf13/cobra"
	"github.com/wwwzy/CentAgent/internal/monitor"
	"github.com/wwwzy/CentAgent/internal/storage"
)

// storageCmd represents the storage command
var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "管理存储和数据库",
	Long:  `提供查看数据库概况、清理监控数据和审计记录的命令。`,
}

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "显示数据库统计概况",
	Run:   runInfo,
}

func init() {
	rootCmd.AddCommand(storageCmd)
	storageCmd.AddCommand(infoCmd)
	storageCmd.AddCommand(pruneMonitorCmd)
	storageCmd.AddCommand(pruneAuditCmd)
}

// pruneAuditCmd represents the prune-audit command
var pruneAuditCmd = &cobra.Command{
	Use:   "prune-audit",
	Short: "清理审计记录",
	Long:  `根据用户指定的保留条数或天数，清理旧的审计记录。`,
	Run:   runPruneAudit,
}

var (
	keepAuditCount int
	keepAuditDays  int
)

func init() {
	pruneAuditCmd.Flags().IntVar(&keepAuditCount, "keep", 0, "保留最近的 N 条记录")
	pruneAuditCmd.Flags().IntVar(&keepAuditDays, "days", 0, "保留最近 N 天的记录")
	
	rootCmd.AddCommand(storageCmd)
	storageCmd.AddCommand(infoCmd)
	storageCmd.AddCommand(pruneMonitorCmd)
	storageCmd.AddCommand(pruneAuditCmd)
}

func runPruneAudit(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	if keepAuditCount <= 0 && keepAuditDays <= 0 {
		fmt.Println("Error: must specify either --keep or --days")
		cmd.Usage()
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Config not loaded")
		os.Exit(1)
	}

	fmt.Println("Opening database...")
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	var deletedCount int64

	if keepAuditCount > 0 {
		fmt.Printf("Pruning audit records, keeping latest %d records...\n", keepAuditCount)
		count, err := store.DeleteAuditRecordsKeepLatest(ctx, keepAuditCount)
		if err != nil {
			fmt.Printf("Error pruning by count: %v\n", err)
			os.Exit(1)
		}
		deletedCount += count
	}

	if keepAuditDays > 0 {
		before := time.Now().UTC().AddDate(0, 0, -keepAuditDays)
		fmt.Printf("Pruning audit records older than %d days (before %s)...\n", keepAuditDays, before.Format(time.RFC3339))
		count, err := store.DeleteAuditRecordsBefore(ctx, before)
		if err != nil {
			fmt.Printf("Error pruning by days: %v\n", err)
			os.Exit(1)
		}
		deletedCount += count
	}

	fmt.Printf("Prune completed. Deleted %d records.\n", deletedCount)
	
	if count, err := store.CountAuditRecords(ctx); err == nil {
		fmt.Printf("Remaining Audit Records: %d\n", count)
	}
}

// pruneMonitorCmd represents the prune-monitor command
var pruneMonitorCmd = &cobra.Command{
	Use:   "prune-monitor",
	Short: "根据配置文件立即精简监控数据",
	Long:  `忽略定时任务间隔，立即执行一次全量的监控数据保留策略清理。读取配置文件中的 monitor.retention 策略。`,
	Run:   runPruneMonitor,
}

func runPruneMonitor(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	if cfg == nil {
		fmt.Println("Config not loaded")
		os.Exit(1)
	}

	fmt.Println("Opening database...")
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	fmt.Println("Starting prune job (this may take a while)...")
	fmt.Printf("Policy: Stats KeepAll=%v, Logs KeepAll=%v\n", cfg.Monitor.Retention.Stats.KeepAll, cfg.Monitor.Retention.Logs.KeepAll)

	// 使用 monitor 包暴露的 Prune 函数
	// 需要引入 "github.com/wwwzy/CentAgent/internal/monitor"
	if err := monitor.Prune(ctx, store, cfg.Monitor.Retention); err != nil {
		fmt.Printf("Prune failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Prune completed successfully.")

	// 可选：显示清理后的计数
	if count, err := store.CountContainerStats(ctx); err == nil {
		fmt.Printf("Remaining Stats: %d\n", count)
	}
	if count, err := store.CountContainerLogs(ctx); err == nil {
		fmt.Printf("Remaining Logs: %d\n", count)
	}
}

func runInfo(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// 确保配置已加载 (rootCmd persistent flag handles this via initConfig,
	// but cfg global variable is populated)
	if cfg == nil {
		fmt.Println("Config not loaded")
		os.Exit(1)
	}

	// 1. 获取数据库文件信息
	dbPath := cfg.Storage.Path
	if !filepath.IsAbs(dbPath) {
		// 尝试转换为绝对路径用于展示（虽然 Open 会处理，但展示绝对路径更友好）
		if absPath, err := filepath.Abs(dbPath); err == nil {
			dbPath = absPath
		}
	}

	var dbSizeStr string
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			dbSizeStr = "Not Found (Will be created on first run)"
		} else {
			dbSizeStr = fmt.Sprintf("Error: %v", err)
		}
	} else {
		sizeMB := float64(info.Size()) / 1024 / 1024
		dbSizeStr = fmt.Sprintf("%.2f MB (%s)", sizeMB, dbPath)
	}

	// 2. 连接数据库
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		// 如果数据库文件不存在，Open 可能会创建它，或者如果目录不存在则报错。
		// 这里如果报错，可能意味着无法连接，仅打印文件信息。
		fmt.Printf("Database File: %s\n", dbSizeStr)
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer store.Close()

	// 3. 获取统计信息
	statsCount, err := store.CountContainerStats(ctx)
	if err != nil {
		fmt.Printf("Error counting stats: %v\n", err)
	}
	logsCount, err := store.CountContainerLogs(ctx)
	if err != nil {
		fmt.Printf("Error counting logs: %v\n", err)
	}
	auditCount, err := store.CountAuditRecords(ctx)
	if err != nil {
		fmt.Printf("Error counting audit records: %v\n", err)
	}

	// 4. 格式化输出
	fmt.Printf("Database File: %s\n\n", dbSizeStr)
	
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "Table\tCount")
	fmt.Fprintln(w, "-----\t-----")
	fmt.Fprintf(w, "ContainerStats\t%d\n", statsCount)
	fmt.Fprintf(w, "ContainerLogs\t%d\n", logsCount)
	fmt.Fprintf(w, "AuditRecords\t%d\n", auditCount)
	w.Flush()
}
