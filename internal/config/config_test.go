package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wwwzy/CentAgent/internal/monitor"
	"github.com/wwwzy/CentAgent/internal/storage"
)

func TestLoad_Defaults(t *testing.T) {
	// 设置必填环境变量，绕过 Validate 检查
	t.Setenv("ARK_API_KEY", "dummy-key")
	t.Setenv("ARK_MODEL_ID", "dummy-model")

	// 测试加载默认值（不提供配置文件）
	cfg, err := Load("")
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// 验证默认值
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "centagent.db", cfg.Storage.Path)
	assert.Equal(t, 30*time.Second, cfg.Monitor.Stats.Interval)
	assert.True(t, cfg.Monitor.Stats.Enabled)
}

func TestLoad_ConfigFile(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	content := []byte(`
log_level: "debug"
ark:
  api_key: "file-key"
  model_id: "file-model"
storage:
  path: "test.db"
  busy_timeout: "10s"
monitor:
  stats:
    enabled: false
    interval: "1m"
`)
	err := os.WriteFile(configFile, content, 0644)
	assert.NoError(t, err)

	// 从文件加载
	cfg, err := Load(configFile)
	assert.NoError(t, err)

	// 验证覆盖值
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "test.db", cfg.Storage.Path)
	assert.Equal(t, 10*time.Second, cfg.Storage.BusyTimeout)
	assert.False(t, cfg.Monitor.Stats.Enabled)
	assert.Equal(t, 1*time.Minute, cfg.Monitor.Stats.Interval)

	// 验证未覆盖的字段保持默认值
	assert.Equal(t, monitor.DefaultConfig().Logs.QueueSize, cfg.Monitor.Logs.QueueSize)
}

func TestLoad_EnvOverride(t *testing.T) {
	// 设置环境变量
	t.Setenv("CENTAGENT_LOG_LEVEL", "warn")
	t.Setenv("CENTAGENT_STORAGE_PATH", "env.db")
	t.Setenv("CENTAGENT_MONITOR_STATS_INTERVAL", "5m")
	// 必须设置必填项，否则 Validate 会失败
	t.Setenv("ARK_API_KEY", "test-key")
	t.Setenv("ARK_MODEL_ID", "test-model")

	// 加载配置（无文件）
	cfg, err := Load("")
	assert.NoError(t, err)

	// 验证环境变量覆盖
	assert.Equal(t, "warn", cfg.LogLevel)
	assert.Equal(t, "env.db", cfg.Storage.Path)
	assert.Equal(t, 5*time.Minute, cfg.Monitor.Stats.Interval)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	// 验证几个关键默认值
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, storage.Config{Path: "centagent.db", BusyTimeout: 5 * time.Second}, cfg.Storage)
	assert.Equal(t, monitor.DefaultConfig().Stats.Interval, cfg.Monitor.Stats.Interval)
}

func TestLoad_ValidateArk(t *testing.T) {
	// 确保没有环境变量干扰
	t.Setenv("ARK_API_KEY", "")
	t.Setenv("ARK_MODEL_ID", "")

	_, err := Load("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ark.api_key is required")
}
