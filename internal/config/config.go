package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/wwwzy/CentAgent/internal/monitor"
	"github.com/wwwzy/CentAgent/internal/storage"
)

type ArkConfig struct {
	APIKey  string `mapstructure:"api_key"`
	ModelID string `mapstructure:"model_id"`
	BaseURL string `mapstructure:"base_url"`
}

type Config struct {
	Storage  storage.Config `mapstructure:"storage"`
	Monitor  monitor.Config `mapstructure:"monitor"`
	Ark      ArkConfig      `mapstructure:"ark"`
	LogLevel string         `mapstructure:"log_level"`
}



func Load(cfgFile string) (*Config, error) {
	// 1. 初始化 Viper
	v := viper.New()

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		// 默认搜索路径
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.centagent")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	v.SetEnvPrefix("CENTAGENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// -------------------------------------------------------------------------
	// 绑定环境变量 (Env Vars)
	// -------------------------------------------------------------------------
	// 如果需要手动绑定环境变量到 key，可以在这里做。
	// 但对于简单情况，AutomaticEnv 已经足够。
	// 注意：为了让 Unmarshal 正确工作（尤其是覆盖已有结构体实例时），
	// Viper 需要知道有哪些 key。它只反序列化它“知道”的 key（来自配置文件、Defaults 或显式 Bind）。
	// 如果一个 key 只存在于环境变量中，Unmarshal 可能会忽略它（特别是当我们 decode 到一个已有数据的结构体时）。

	// 正确的做法：让 Viper 解码到结构体中。
	// 但是 Viper 的 Unmarshal 本身不会把 `cfg` 里的现有值当作默认值，
	// 除非我们显式地通过 v.SetDefault 设置它们。

	setDefaults(v)

	// 2. 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
		// 配置文件未找到，使用默认值
	}

	// 3. 反序列化 (文件/环境变量 覆盖 默认值)
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 4. 验证关键配置
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	// Ark 配置验证：必须存在
	if c.Ark.APIKey == "" {
		return fmt.Errorf("ark.api_key is required (or set ARK_API_KEY env var)")
	}
	if c.Ark.ModelID == "" {
		return fmt.Errorf("ark.model_id is required (or set ARK_MODEL_ID env var)")
	}
	return nil
}

func setDefaults(v *viper.Viper) {
	// -------------------------------------------------------------------------
	// Global Defaults (全局默认值)
	// -------------------------------------------------------------------------
	v.SetDefault("log_level", "info")

	// -------------------------------------------------------------------------
	// Storage Defaults (存储默认值)
	// -------------------------------------------------------------------------
	v.SetDefault("storage.path", "centagent.db")
	v.SetDefault("storage.busy_timeout", 5*time.Second)

	// -------------------------------------------------------------------------
	// Monitor Stats Defaults (状态采集默认值)
	// -------------------------------------------------------------------------
	monitorDefaults := monitor.DefaultConfig()
	v.SetDefault("monitor.stats.enabled", monitorDefaults.Stats.Enabled)
	v.SetDefault("monitor.stats.interval", monitorDefaults.Stats.Interval)
	v.SetDefault("monitor.stats.workers", monitorDefaults.Stats.Workers)
	v.SetDefault("monitor.stats.queue_size", monitorDefaults.Stats.QueueSize)
	v.SetDefault("monitor.stats.batch_size", monitorDefaults.Stats.BatchSize)
	v.SetDefault("monitor.stats.flush_interval", monitorDefaults.Stats.FlushInterval)
	v.SetDefault("monitor.stats.max_raw_json_bytes", monitorDefaults.Stats.MaxRawJSONBytes)

	// -------------------------------------------------------------------------
	// Monitor Logs Defaults (日志采集默认值)
	// -------------------------------------------------------------------------
	v.SetDefault("monitor.logs.enabled", monitorDefaults.Logs.Enabled)
	v.SetDefault("monitor.logs.queue_size", monitorDefaults.Logs.QueueSize)
	v.SetDefault("monitor.logs.batch_size", monitorDefaults.Logs.BatchSize)
	v.SetDefault("monitor.logs.flush_interval", monitorDefaults.Logs.FlushInterval)
	v.SetDefault("monitor.logs.max_line_bytes", monitorDefaults.Logs.MaxLineBytes)
	v.SetDefault("monitor.logs.tailer_limit", monitorDefaults.Logs.TailerLimit)
	v.SetDefault("monitor.logs.since_from_start", monitorDefaults.Logs.SinceFromStart)
	v.SetDefault("monitor.logs.reconnect_delay", monitorDefaults.Logs.ReconnectDelay)
	v.SetDefault("monitor.logs.reconnect_jitter", monitorDefaults.Logs.ReconnectJitter)

	// -------------------------------------------------------------------------
	// Monitor Retention Defaults (数据清理默认值)
	// -------------------------------------------------------------------------
	v.SetDefault("monitor.retention.enabled", monitorDefaults.Retention.Enabled)
	v.SetDefault("monitor.retention.interval", monitorDefaults.Retention.Interval)
	v.SetDefault("monitor.retention.workers", monitorDefaults.Retention.Workers)
	v.SetDefault("monitor.retention.batch_rows", monitorDefaults.Retention.BatchRows)
	v.SetDefault("monitor.retention.idle_sleep", monitorDefaults.Retention.IdleSleep)

	// Retention Stats Policy
	v.SetDefault("monitor.retention.stats.keep_all", monitorDefaults.Retention.Stats.KeepAll)
	v.SetDefault("monitor.retention.stats.keep_anomaly_until", monitorDefaults.Retention.Stats.KeepAnomalyUntil)
	v.SetDefault("monitor.retention.stats.cpu_high", monitorDefaults.Retention.Stats.CPUHigh)
	v.SetDefault("monitor.retention.stats.mem_high", monitorDefaults.Retention.Stats.MemHigh)

	// Retention Logs Policy
	v.SetDefault("monitor.retention.logs.keep_all", monitorDefaults.Retention.Logs.KeepAll)
	v.SetDefault("monitor.retention.logs.keep_important_until", monitorDefaults.Retention.Logs.KeepImportantUntil)

	// -------------------------------------------------------------------------
	// Ark AI Defaults (AI 模型默认值)
	// -------------------------------------------------------------------------
	// 尝试从环境变量读取默认值，如果环境变量存在
	v.SetDefault("ark.api_key", "")
	v.SetDefault("ark.model_id", "")
	v.SetDefault("ark.base_url", "https://ark.cn-beijing.volces.com/api/v3")

	v.BindEnv("ark.api_key", "ARK_API_KEY")
	v.BindEnv("ark.model_id", "ARK_MODEL_ID")
	v.BindEnv("ark.base_url", "ARK_BASE_URL")
}

func DefaultConfig() Config {
	return Config{
		LogLevel: "info",
		Storage: storage.Config{
			Path:        "centagent.db",
			BusyTimeout: 5 * time.Second,
		},
		Monitor: monitor.DefaultConfig(),
	}
}
