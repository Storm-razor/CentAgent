package storage

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	defaultLimit = 200
	maxLimit     = 5000

	defaultDeleteLimit = 500
	maxDeleteLimit     = 900
)

type StatsQuery struct {
	// ContainerID/ContainerName 为可选过滤条件，均为精确匹配；通常优先使用 ContainerID（更稳定）。
	ContainerID   string
	ContainerName string
	// From/To 过滤 CollectedAt 区间：[From, To]（两端包含）。
	From *time.Time
	To   *time.Time
	// Limit 限制返回条数；<=0 使用默认值。
	Limit int
	// Desc 按 CollectedAt 倒序返回（优先返回最新采样点）。
	Desc bool
}

func (s *Storage) InsertContainerStat(ctx context.Context, stat *ContainerStat) error {
	if s == nil || s.db == nil {
		return errors.New("storage not initialized")
	}
	if stat == nil {
		return errors.New("stat is nil")
	}
	now := time.Now().UTC()
	if stat.CollectedAt.IsZero() {
		stat.CollectedAt = now
	}
	if stat.CreatedAt.IsZero() {
		stat.CreatedAt = now
	}
	if err := s.db.WithContext(ctx).Create(stat).Error; err != nil {
		return fmt.Errorf("insert container stat: %w", err)
	}
	return nil
}

func (s *Storage) InsertContainerStats(ctx context.Context, stats []ContainerStat) error {
	if s == nil || s.db == nil {
		return errors.New("storage not initialized")
	}
	if len(stats) == 0 {
		return nil
	}
	now := time.Now().UTC()
	for i := range stats {
		if stats[i].CollectedAt.IsZero() {
			stats[i].CollectedAt = now
		}
		if stats[i].CreatedAt.IsZero() {
			stats[i].CreatedAt = now
		}
	}
	if err := s.db.WithContext(ctx).CreateInBatches(stats, 200).Error; err != nil {
		return fmt.Errorf("insert container stats: %w", err)
	}
	return nil
}

func (s *Storage) QueryContainerStats(ctx context.Context, q StatsQuery) ([]ContainerStat, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("storage not initialized")
	}

	limit := normalizeLimit(q.Limit)
	db := s.db.WithContext(ctx).Model(&ContainerStat{})
	if q.ContainerID != "" {
		db = db.Where("container_id = ?", q.ContainerID)
	}
	if q.ContainerName != "" {
		db = db.Where("container_name = ?", q.ContainerName)
	}
	if q.From != nil {
		db = db.Where("collected_at >= ?", *q.From)
	}
	if q.To != nil {
		db = db.Where("collected_at <= ?", *q.To)
	}
	if q.Desc {
		db = db.Order("collected_at DESC")
	} else {
		db = db.Order("collected_at ASC")
	}
	db = db.Limit(limit)

	var out []ContainerStat
	if err := db.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("query container stats: %w", err)
	}
	return out, nil
}

func (s *Storage) CountContainerStats(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialized")
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&ContainerStat{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count container stats: %w", err)
	}
	return count, nil
}

func (s *Storage) DeleteContainerStatsBefore(ctx context.Context, before time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialized")
	}
	res := s.db.WithContext(ctx).Where("collected_at < ?", before).Delete(&ContainerStat{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete container stats: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (s *Storage) DeleteContainerStatsBeforeLimited(ctx context.Context, before time.Time, limit int) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialized")
	}

	limit = normalizeDeleteLimit(limit)

	var ids []uint64
	db := s.db.WithContext(ctx).Model(&ContainerStat{}).
		Select("id").
		Where("collected_at < ?", before).
		Order("id ASC").
		Limit(limit)
	if err := db.Find(&ids).Error; err != nil {
		return 0, fmt.Errorf("select container stats ids: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	res := s.db.WithContext(ctx).Where("id IN ?", ids).Delete(&ContainerStat{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete container stats: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (s *Storage) DeleteContainerStatsNonAnomalyInRangeLimited(ctx context.Context, from time.Time, to time.Time, cpuHigh float64, memHigh float64, limit int) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialized")
	}
	if !to.After(from) {
		return 0, nil
	}

	limit = normalizeDeleteLimit(limit)

	db := s.db.WithContext(ctx).Model(&ContainerStat{}).
		Select("id").
		Where("collected_at >= ? AND collected_at < ?", from, to)
	if cpuHigh > 0 {
		db = db.Where("cpu_percent < ?", cpuHigh)
	}
	if memHigh > 0 {
		db = db.Where("mem_percent < ?", memHigh)
	}

	var ids []uint64
	if err := db.Order("id ASC").Limit(limit).Find(&ids).Error; err != nil {
		return 0, fmt.Errorf("select container stats ids: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	res := s.db.WithContext(ctx).Where("id IN ?", ids).Delete(&ContainerStat{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete container stats: %w", res.Error)
	}
	return res.RowsAffected, nil
}

type LogQuery struct {
	// ContainerID/ContainerName 为可选过滤条件，均为精确匹配；通常优先使用 ContainerID（更稳定）。
	ContainerID   string
	ContainerName string
	// From/To 过滤 Timestamp 区间：[From, To]（两端包含）。
	From *time.Time
	To   *time.Time
	// Level/Source 为可选过滤条件，均为精确匹配（Level 如 ERROR；Source 如 stdout/stderr）。
	Level  string
	Source string
	// Contains 对 Message 做子串匹配（SQL LIKE），用于关键字检索。
	Contains string
	// Limit 限制返回条数；<=0 使用默认值。
	Limit int
	// Desc 按 Timestamp 倒序返回（优先返回最新日志）。
	Desc bool
}

func (s *Storage) InsertContainerLog(ctx context.Context, log *ContainerLog) error {
	if s == nil || s.db == nil {
		return errors.New("storage not initialized")
	}
	if log == nil {
		return errors.New("log is nil")
	}
	now := time.Now().UTC()
	if log.Timestamp.IsZero() {
		log.Timestamp = now
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = now
	}
	if err := s.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("insert container log: %w", err)
	}
	return nil
}

func (s *Storage) InsertContainerLogs(ctx context.Context, logs []ContainerLog) error {
	if s == nil || s.db == nil {
		return errors.New("storage not initialized")
	}
	if len(logs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	for i := range logs {
		if logs[i].Timestamp.IsZero() {
			logs[i].Timestamp = now
		}
		if logs[i].CreatedAt.IsZero() {
			logs[i].CreatedAt = now
		}
	}
	if err := s.db.WithContext(ctx).CreateInBatches(logs, 200).Error; err != nil {
		return fmt.Errorf("insert container logs: %w", err)
	}
	return nil
}

func (s *Storage) QueryContainerLogs(ctx context.Context, q LogQuery) ([]ContainerLog, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("storage not initialized")
	}

	limit := normalizeLimit(q.Limit)
	db := s.db.WithContext(ctx).Model(&ContainerLog{})
	if q.ContainerID != "" {
		db = db.Where("container_id = ?", q.ContainerID)
	}
	if q.ContainerName != "" {
		db = db.Where("container_name = ?", q.ContainerName)
	}
	if q.Source != "" {
		db = db.Where("source = ?", q.Source)
	}
	if q.Level != "" {
		db = db.Where("level = ?", q.Level)
	}
	if q.From != nil {
		db = db.Where("timestamp >= ?", *q.From)
	}
	if q.To != nil {
		db = db.Where("timestamp <= ?", *q.To)
	}
	if q.Contains != "" {
		db = db.Where("message LIKE ?", "%"+q.Contains+"%")
	}
	if q.Desc {
		db = db.Order("timestamp DESC")
	} else {
		db = db.Order("timestamp ASC")
	}
	db = db.Limit(limit)

	var out []ContainerLog
	if err := db.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("query container logs: %w", err)
	}
	return out, nil
}

func (s *Storage) CountContainerLogs(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialized")
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&ContainerLog{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count container logs: %w", err)
	}
	return count, nil
}

func (s *Storage) DeleteContainerLogsBefore(ctx context.Context, before time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialized")
	}
	res := s.db.WithContext(ctx).Where("timestamp < ?", before).Delete(&ContainerLog{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete container logs: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (s *Storage) DeleteContainerLogsBeforeLimited(ctx context.Context, before time.Time, limit int) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialized")
	}

	limit = normalizeDeleteLimit(limit)

	var ids []uint64
	db := s.db.WithContext(ctx).Model(&ContainerLog{}).
		Select("id").
		Where("timestamp < ?", before).
		Order("id ASC").
		Limit(limit)
	if err := db.Find(&ids).Error; err != nil {
		return 0, fmt.Errorf("select container logs ids: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	res := s.db.WithContext(ctx).Where("id IN ?", ids).Delete(&ContainerLog{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete container logs: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (s *Storage) DeleteContainerLogsUnimportantInRangeLimited(ctx context.Context, from time.Time, to time.Time, keepLevels []string, keepSources []string, limit int) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialized")
	}
	if !to.After(from) {
		return 0, nil
	}

	limit = normalizeDeleteLimit(limit)

	db := s.db.WithContext(ctx).Model(&ContainerLog{}).
		Select("id").
		Where("timestamp >= ? AND timestamp < ?", from, to)
	if len(keepLevels) > 0 {
		db = db.Where("level NOT IN ?", keepLevels)
	}
	if len(keepSources) > 0 {
		db = db.Where("source NOT IN ?", keepSources)
	}

	var ids []uint64
	if err := db.Order("id ASC").Limit(limit).Find(&ids).Error; err != nil {
		return 0, fmt.Errorf("select container logs ids: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	res := s.db.WithContext(ctx).Where("id IN ?", ids).Delete(&ContainerLog{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete container logs: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// AuditQuery 用于查询审计记录的过滤条件。
//
// 设计原则：
//   - 所有字段都是“可选过滤条件”，零值表示不参与过滤。
//   - 时间范围使用 CreatedAt（写入时间），用于“最近 N 次操作/某段时间内发生了什么”这类审计检索。
type AuditQuery struct {
	// TraceID 精确匹配链路 ID。
	TraceID string
	// Action 精确匹配动作名（建议为稳定的工具名/命令名）。
	Action string
	// Status 精确匹配执行状态（例如 running/success/failed）。
	Status string
	// From/To 过滤 CreatedAt 区间：[From, To]（两端包含）。
	From *time.Time
	To   *time.Time
	// Limit 限制返回条数；<=0 使用默认值。
	Limit int
	// Desc 按 CreatedAt 倒序返回（优先返回最新记录）。
	Desc bool
}

func (s *Storage) InsertAuditRecord(ctx context.Context, rec *AuditRecord) error {
	if s == nil || s.db == nil {
		return errors.New("storage not initialized")
	}
	if rec == nil {
		return errors.New("audit record is nil")
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if err := s.db.WithContext(ctx).Create(rec).Error; err != nil {
		return fmt.Errorf("insert audit record: %w", err)
	}
	return nil
}

func (s *Storage) QueryAuditRecords(ctx context.Context, q AuditQuery) ([]AuditRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("storage not initialized")
	}

	limit := normalizeLimit(q.Limit)
	db := s.db.WithContext(ctx).Model(&AuditRecord{})
	if q.TraceID != "" {
		db = db.Where("trace_id = ?", q.TraceID)
	}
	if q.Action != "" {
		db = db.Where("action = ?", q.Action)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	if q.From != nil {
		db = db.Where("created_at >= ?", *q.From)
	}
	if q.To != nil {
		db = db.Where("created_at <= ?", *q.To)
	}
	if q.Desc {
		db = db.Order("created_at DESC")
	} else {
		db = db.Order("created_at ASC")
	}
	db = db.Limit(limit)

	var out []AuditRecord
	if err := db.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("query audit records: %w", err)
	}
	return out, nil
}

func (s *Storage) CountAuditRecords(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialized")
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&AuditRecord{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count audit records: %w", err)
	}
	return count, nil
}

type AuditUpdate struct {
	Status       *string
	ResultJSON   *string
	ErrorMessage *string
	FinishedAt   *time.Time
}

func (s *Storage) UpdateAuditRecord(ctx context.Context, id uint64, up AuditUpdate) error {
	if s == nil || s.db == nil {
		return errors.New("storage not initialized")
	}

	updates := make(map[string]interface{})
	if up.Status != nil {
		updates["status"] = *up.Status
	}
	if up.ResultJSON != nil {
		updates["result_json"] = *up.ResultJSON
	}
	if up.ErrorMessage != nil {
		updates["error_message"] = *up.ErrorMessage
	}
	if up.FinishedAt != nil {
		updates["finished_at"] = *up.FinishedAt
	}

	if len(updates) == 0 {
		return nil
	}

	res := s.db.WithContext(ctx).Model(&AuditRecord{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("update audit record: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gormNotFoundError("audit record", id)
	}
	return nil
}

func (s *Storage) DeleteAuditRecordsKeepLatest(ctx context.Context, keepCount int) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialized")
	}
	if keepCount < 0 {
		return 0, errors.New("keep count must be non-negative")
	}

	// 1. 查找第 keepCount+1 条记录的 ID（按时间倒序）
	// 如果总数 <= keepCount，子查询返回空，不会删除任何内容
	// SQLite 支持 LIMIT/OFFSET，这里我们找到“分界线 ID”
	var boundaryID uint64
	err := s.db.WithContext(ctx).Model(&AuditRecord{}).
		Select("id").
		Order("created_at DESC").
		Offset(keepCount).
		Limit(1).
		Scan(&boundaryID).Error

	if err != nil {
		return 0, fmt.Errorf("find audit boundary id: %w", err)
	}

	if boundaryID == 0 {
		// 说明记录数不足 keepCount，无需删除
		return 0, nil
	}

	// 2. 删除 ID <= boundaryID 的所有记录（假设 ID 是自增且大致随时间递增的，或者直接用 CreatedAt）
	// 更严谨的做法是：删除那些 CreatedAt <= (boundaryRecord.CreatedAt) 且 ID != boundaryRecord.ID ...
	// 但通常 AuditLog 只追加，ID 大小与时间正相关。
	// 为了安全起见，我们还是用 ID 列表或者“不在前 N 个 ID 集合中”的方式。
	// DELETE FROM audit_records WHERE id NOT IN (SELECT id FROM audit_records ORDER BY created_at DESC LIMIT ?)

	res := s.db.WithContext(ctx).Where("id <= ?", boundaryID).Delete(&AuditRecord{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete audit records: %w", res.Error)
	}

	return res.RowsAffected, nil
}

func (s *Storage) DeleteAuditRecordsBefore(ctx context.Context, before time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialized")
	}
	res := s.db.WithContext(ctx).Where("created_at < ?", before).Delete(&AuditRecord{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete audit records: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func normalizeLimit(v int) int {
	if v <= 0 {
		return defaultLimit
	}
	if v > maxLimit {
		return maxLimit
	}
	return v
}

func normalizeDeleteLimit(v int) int {
	if v <= 0 {
		return defaultDeleteLimit
	}
	if v > maxDeleteLimit {
		return maxDeleteLimit
	}
	return v
}

type notFoundError struct {
	Entity string
	ID     uint64
}

func (e notFoundError) Error() string {
	return fmt.Sprintf("%s not found: %d", e.Entity, e.ID)
}

func gormNotFoundError(entity string, id uint64) error {
	return notFoundError{Entity: entity, ID: id}
}
