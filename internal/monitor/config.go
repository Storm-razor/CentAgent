package monitor

import (
	"runtime"
	"time"
)

type ErrorHandler func(err error)

type StatsConfig struct {
	// Enabled 控制 Stats 采集流水线是否启用。
	Enabled bool

	// Interval 为采集周期；每到一个周期会扫描容器列表并触发一次采样。
	Interval time.Duration
	// Workers 为并发采样的 worker 数量；每个 worker 负责从队列取容器并采集一次 stats。
	Workers int
	// QueueSize 为待采样容器队列的缓冲大小；容器数量较多时可适当增大以减少阻塞。
	QueueSize int
	// BatchSize 为单次写入数据库的最大批量；达到批量即触发一次落库。
	BatchSize int
	// FlushInterval 为写入端的最大等待时间；即使未达到 BatchSize，也会按该间隔定时落库。
	FlushInterval time.Duration

	// MaxRawJSONBytes 限制落库时 RawJSON 的最大长度（字节）；超过则写入 {"_truncated":true}。
	MaxRawJSONBytes int

	// OnError 为异步错误回调（例如采样失败、落库失败、列容器失败）；默认丢弃。
	OnError ErrorHandler
}

type Config struct {
	Stats StatsConfig
}

func DefaultConfig() Config {
	return Config{
		Stats: StatsConfig{
			Enabled:         true,
			Interval:        10 * time.Second,
			Workers:         max(2, runtime.NumCPU()),
			QueueSize:       256,
			BatchSize:       100,
			FlushInterval:   2 * time.Second,
			MaxRawJSONBytes: 1024,
		},
	}
}

func (c StatsConfig) withDefaults() StatsConfig {
	if c.Interval <= 0 {
		c.Interval = 10 * time.Second
	}
	if c.Workers <= 0 {
		c.Workers = max(2, runtime.NumCPU())
	}
	if c.QueueSize <= 0 {
		c.QueueSize = 256
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 100
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = 2 * time.Second
	}
	if c.MaxRawJSONBytes <= 0 {
		c.MaxRawJSONBytes = 128 * 1024
	}
	if c.OnError == nil {
		c.OnError = func(error) {}
	}
	return c
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
