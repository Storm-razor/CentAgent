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

// StatsRetentionPolicy 定义 stats（容器状态采样）数据的分层保留策略。
type StatsRetentionPolicy struct {
	// KeepAll 为全量保留窗口；在该窗口内的 stats 全部保留，不做删除。
	KeepAll time.Duration
	// KeepAnomalyUntil 为“异常保留”窗口上界；超过该窗口的 stats 全部清除；
	// 在 [KeepAll, KeepAnomalyUntil) 区间内，仅保留 CPU/Mem 过高的采样点。
	KeepAnomalyUntil time.Duration
	// CPUHigh/MemHigh 为异常阈值（百分比）；满足 CPUPercent>=CPUHigh 或 MemPercent>=MemHigh 视为异常。
	CPUHigh float64
	MemHigh float64
}

// LogsRetentionPolicy 定义 logs（容器日志）数据的分层保留策略。
type LogsRetentionPolicy struct {
	// KeepAll 为全量保留窗口；在该窗口内的日志全部保留，不做删除。
	KeepAll time.Duration
	// KeepImportantUntil 为“重要日志保留”窗口上界；超过该窗口的日志全部清除；
	// 在 [KeepAll, KeepImportantUntil) 区间内，仅保留重要等级/来源的日志。
	KeepImportantUntil time.Duration
	// KeepLevels 为重要日志等级白名单（例如 ERROR/WARN）；为空表示不按等级做保留。
	KeepLevels []string
	// KeepSources 为重要来源白名单（例如 stderr）；为空表示不按来源做保留。
	KeepSources []string
}

// RetentionConfig 为自动清理（分层删除）流水线的配置。
type RetentionConfig struct {
	// Enabled 控制自动清理过期 stats/logs 流水线是否启用。
	Enabled bool

	// Interval 为清理周期；每到一个周期会执行一次分层保留策略的清理。
	Interval time.Duration
	// Workers 为并发清理的 worker 数量；每个 worker 负责执行一类清理任务。
	Workers int
	// BatchRows 为单次删除的最大行数；用于分批删除以降低长事务/写锁影响。
	BatchRows int
	// IdleSleep 为每批删除后的短暂等待；用于降低持续写锁对采集写入的影响。
	IdleSleep time.Duration

	// Stats/Logs 分别定义状态采样与日志的分层保留策略。
	Stats StatsRetentionPolicy
	Logs  LogsRetentionPolicy

	// OnError 为异步错误回调（例如删除失败、配置非法等）；默认丢弃。
	OnError ErrorHandler
}

type Config struct {
	Stats     StatsConfig
	Logs      LogConfig
	Retention RetentionConfig
}

func DefaultConfig() Config {
	return Config{
		Stats: StatsConfig{
			Enabled:         true,
			Interval:        30 * time.Second,
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
		Retention: RetentionConfig{
			Enabled:   true,
			Interval:  1 * time.Hour,
			Workers:   2,
			BatchRows: 500,
			IdleSleep: 25 * time.Millisecond,
			Stats: StatsRetentionPolicy{
				KeepAll:          12 * time.Hour,
				KeepAnomalyUntil: 5 * 24 * time.Hour,
				CPUHigh:          80,
				MemHigh:          80,
			},
			Logs: LogsRetentionPolicy{
				KeepAll:            12 * time.Hour,
				KeepImportantUntil: 5 * 24 * time.Hour,
				KeepLevels:         []string{"ERROR", "WARN"},
				KeepSources:        []string{"stderr"},
			},
		},
	}
}

func (c StatsConfig) withDefaults() StatsConfig {
	if c.Interval <= 0 {
		c.Interval = 30 * time.Second
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

func (c RetentionConfig) withDefaults() RetentionConfig {
	if c.Interval <= 0 {
		c.Interval = 1 * time.Hour
	}
	if c.Workers <= 0 {
		c.Workers = 2
	}
	if c.BatchRows <= 0 {
		c.BatchRows = 500
	}
	if c.BatchRows > 900 {
		c.BatchRows = 900
	}
	if c.IdleSleep < 0 {
		c.IdleSleep = 0
	}
	if c.Stats.KeepAll <= 0 {
		c.Stats.KeepAll = 12 * time.Hour
	}
	if c.Stats.KeepAnomalyUntil <= 0 {
		c.Stats.KeepAnomalyUntil = 5 * 24 * time.Hour
	}
	if c.Stats.KeepAnomalyUntil < c.Stats.KeepAll {
		c.Stats.KeepAnomalyUntil = c.Stats.KeepAll
	}
	if c.Stats.CPUHigh < 0 {
		c.Stats.CPUHigh = 0
	}
	if c.Stats.MemHigh < 0 {
		c.Stats.MemHigh = 0
	}

	if c.Logs.KeepAll <= 0 {
		c.Logs.KeepAll = 12 * time.Hour
	}
	if c.Logs.KeepImportantUntil <= 0 {
		c.Logs.KeepImportantUntil = 5 * 24 * time.Hour
	}
	if c.Logs.KeepImportantUntil < c.Logs.KeepAll {
		c.Logs.KeepImportantUntil = c.Logs.KeepAll
	}
	if c.OnError == nil {
		c.OnError = func(error) {}
	}
	return c
}
