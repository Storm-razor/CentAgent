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

type LogConfig struct {
	// Enabled 控制日志收集流水线是否启用（Events + Follow + 落库）。
	Enabled bool

	// QueueSize 为解析后日志记录的缓冲队列大小；队列满时会触发丢弃并回调 OnError。
	QueueSize int
	// BatchSize 为单次写入数据库的最大批量；达到批量即触发一次落库。
	BatchSize int
	// FlushInterval 为写入端的最大等待时间；即使未达到 BatchSize，也会按该间隔定时落库。
	FlushInterval time.Duration

	// MaxLineBytes 为单行日志的最大长度（字节）；超过会导致扫描报错并结束该容器 tailer。
	MaxLineBytes int
	// TailerLimit 为最多允许同时运行的 tailer 数（每容器一个）；超过则拒绝新增并回调 OnError。
	TailerLimit int
	// SinceFromStart 为 true 时，仅收集从 collector 启动时刻起的新日志（避免历史日志灌库）。
	SinceFromStart bool
	// ReconnectDelay 为 Events 断开后的基础重连间隔。
	ReconnectDelay time.Duration
	// ReconnectJitter 为重连抖动区间（±jitter），用于降低重连风暴风险。
	ReconnectJitter time.Duration

	// OnError 为异步错误回调（例如 events 断开、tailer 启动失败、队列满等）；默认丢弃。
	OnError ErrorHandler
}

type Config struct {
	Stats StatsConfig
	Logs  LogConfig
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
		Logs: LogConfig{
			Enabled:         false,
			QueueSize:       1024,
			BatchSize:       200,
			FlushInterval:   2 * time.Second,
			MaxLineBytes:    64 * 1024,
			TailerLimit:     128,
			SinceFromStart:  true,
			ReconnectDelay:  2 * time.Second,
			ReconnectJitter: 500 * time.Millisecond,
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

func (c LogConfig) withDefaults() LogConfig {
	if c.QueueSize <= 0 {
		c.QueueSize = 1024
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 200
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = 2 * time.Second
	}
	if c.MaxLineBytes <= 0 {
		c.MaxLineBytes = 64 * 1024
	}
	if c.TailerLimit <= 0 {
		c.TailerLimit = 128
	}
	if c.ReconnectDelay <= 0 {
		c.ReconnectDelay = 2 * time.Second
	}
	if c.ReconnectJitter < 0 {
		c.ReconnectJitter = 0
	}
	if c.OnError == nil {
		c.OnError = func(error) {}
	}
	return c
}
