package storage

import "time"

type ContainerStat struct {
	// ID 为自增主键（内部使用）。
	ID uint64 `gorm:"primaryKey"`
	// ContainerID 为容器唯一标识（Docker ID），用于跨重启/重命名保持稳定关联。
	ContainerID string `gorm:"size:128;not null;index:idx_container_stats_container_time,priority:1"`
	// ContainerName 为采样时刻的容器名称（可变），便于展示与按名称检索。
	ContainerName string `gorm:"size:255;index"`
	// CPUPercent 为 CPU 使用率百分比（0~100+，取决于核数与计算方式），用于趋势分析与告警。
	CPUPercent float64 `gorm:"not null"`
	// MemUsageBytes/MemLimitBytes 为内存使用/限制（字节），用于计算 MemPercent 与趋势分析。
	MemUsageBytes uint64 `gorm:"not null"`
	MemLimitBytes uint64 `gorm:"not null"`
	// MemPercent 为内存使用率百分比（0~1 或 0~100，取决于上游采集口径；建议保持一致）。
	MemPercent float64 `gorm:"not null"`
	// NetRxBytes/NetTxBytes 为网络收发累计字节数（采样点读数），用于计算速率与趋势。
	NetRxBytes uint64 `gorm:"not null"`
	NetTxBytes uint64 `gorm:"not null"`
	// BlockReadBytes/BlockWriteBytes 为块设备读写累计字节数（采样点读数）。
	BlockReadBytes  uint64 `gorm:"not null"`
	BlockWriteBytes uint64 `gorm:"not null"`
	// Pids 为容器内进程数（采样点读数）。
	Pids uint64 `gorm:"not null"`
	// RawJSON 可选：存放采样的原始 JSON，便于未来字段扩展或离线重算。
	RawJSON string `gorm:"type:text"`
	// CollectedAt 为采样发生时间（推荐用 UTC），用于时序查询与聚合；与 ContainerID 组成联合索引。
	CollectedAt time.Time `gorm:"not null;index:idx_container_stats_container_time,priority:2"`
	// CreatedAt 为写入数据库时间（与 CollectedAt 含义不同），默认自动填充。
	CreatedAt time.Time `gorm:"not null;autoCreateTime"`
}

// ContainerLog 表示一个被持久化的“日志片段/日志事件”。
//
// 该表面向两类需求：
//   - 查询分析：按容器、时间范围、级别、关键字检索最近日志（例如排障）。
//   - 轻量存储：不追求完整无限增长的日志仓库，通常配合“按时间清理”做保留策略。
type ContainerLog struct {
	// ID 为自增主键（内部使用）。
	ID uint64 `gorm:"primaryKey"`
	// ContainerID 为容器唯一标识（Docker ID），用于稳定关联；与 Timestamp 组成联合索引。
	ContainerID string `gorm:"size:128;not null;index:idx_container_logs_container_time,priority:1"`
	// ContainerName 为采集时刻的容器名称（可变），便于展示与按名称检索。
	ContainerName string `gorm:"size:255;index"`
	// Source 表示日志来源（stdout/stderr 等），用于快速筛选。
	Source string `gorm:"size:16;not null;index"`
	// Level 为解析出的日志级别（可选：INFO/WARN/ERROR...），便于筛选与统计。
	Level string `gorm:"size:32;index"`
	// Message 为主要可读内容（建议是已解码后的单条或短片段）。
	Message string `gorm:"type:text;not null"`
	// Timestamp 为日志发生时间（推荐用 UTC）；与 ContainerID 组成联合索引。
	Timestamp time.Time `gorm:"not null;index:idx_container_logs_container_time,priority:2"`
	// Raw 可选：原始日志行/原始 payload，便于回溯或重新解析。
	Raw string `gorm:"type:text"`
	// CreatedAt 为写入数据库时间（与 Timestamp 含义不同），默认自动填充。
	CreatedAt time.Time `gorm:"not null;autoCreateTime"`
}

// AuditRecord 记录一次“对系统的操作”及其结果，用于审计、追溯与后续分析。
//
// 一条审计记录通常对应一次 Agent/CLI 的意图执行（例如：列出容器、重启容器、拉取镜像）。
// 它不要求保存大对象的结构化字段，复杂入参/输出统一以 JSON 字符串存放，便于快速落地与版本演进。
type AuditRecord struct {
	// ID 为自增主键（内部使用）。
	ID uint64 `gorm:"primaryKey"`
	// TraceID 用于串联一次请求/对话/编排链路（可选），便于按链路聚合审计。
	TraceID string `gorm:"size:64;index"`
	// Action 表示执行的动作（建议是稳定的“工具名/命令名”，例如 docker.ps / docker.restart）。
	Action string `gorm:"size:128;not null;index"`
	// ParamsJSON 存放动作入参（JSON 字符串），例如工具调用参数。
	ParamsJSON string `gorm:"type:text"`
	// ResultJSON 存放动作结果（JSON 字符串），例如工具输出摘要/结构化结果。
	ResultJSON string `gorm:"type:text"`
	// Status 表示执行状态（例如 running/success/failed），用于快速筛选与统计。
	Status string `gorm:"size:32;not null;index"`
	// ErrorMessage 存放失败时的错误信息（可选，便于检索）。
	ErrorMessage string `gorm:"type:text"`
	// StartedAt/FinishedAt 表示动作起止时间（可选）。统计耗时可用 FinishedAt-StartedAt。
	StartedAt  time.Time `gorm:"index"`
	FinishedAt time.Time `gorm:"index"`
	// CreatedAt 为记录写入数据库的时间（与 StartedAt 含义不同），默认自动填充。
	CreatedAt time.Time `gorm:"not null;autoCreateTime;index"`
}
