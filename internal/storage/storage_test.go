package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTestStorage(t *testing.T) *Storage {
	t.Helper()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "centagent.db")
	s, err := Open(ctx, Config{
		Path:      dbPath,
		EnableWAL: true,
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestContainerStatsRoundtrip(t *testing.T) {
	s := openTestStorage(t)
	ctx := context.Background()

	base := time.Now().Add(-10 * time.Minute).UTC()
	a1 := ContainerStat{
		ContainerID:   "cid-a",
		ContainerName: "nginx-a",
		CPUPercent:    1.2,
		MemUsageBytes: 123,
		MemLimitBytes: 456,
		MemPercent:    0.27,
		CollectedAt:   base,
	}
	a2 := ContainerStat{
		ContainerID:   "cid-a",
		ContainerName: "nginx-a",
		CPUPercent:    2.3,
		MemUsageBytes: 124,
		MemLimitBytes: 456,
		MemPercent:    0.28,
		CollectedAt:   base.Add(2 * time.Minute),
	}
	b1 := ContainerStat{
		ContainerID:   "cid-b",
		ContainerName: "redis-b",
		CPUPercent:    9.9,
		MemUsageBytes: 999,
		MemLimitBytes: 1000,
		MemPercent:    0.99,
		CollectedAt:   base.Add(1 * time.Minute),
	}

	if err := s.InsertContainerStat(ctx, &a1); err != nil {
		t.Fatalf("insert a1: %v", err)
	}
	if err := s.InsertContainerStat(ctx, &a2); err != nil {
		t.Fatalf("insert a2: %v", err)
	}
	if err := s.InsertContainerStat(ctx, &b1); err != nil {
		t.Fatalf("insert b1: %v", err)
	}

	from := base.Add(-30 * time.Second)
	to := base.Add(3 * time.Minute)
	got, err := s.QueryContainerStats(ctx, StatsQuery{
		ContainerID: "cid-a",
		From:        &from,
		To:          &to,
		Limit:       10,
		Desc:        false,
	})
	if err != nil {
		t.Fatalf("query stats: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(got))
	}
	if !got[0].CollectedAt.Equal(a1.CollectedAt) || !got[1].CollectedAt.Equal(a2.CollectedAt) {
		t.Fatalf("unexpected collected_at order: %v then %v", got[0].CollectedAt, got[1].CollectedAt)
	}

	affected, err := s.DeleteContainerStatsBefore(ctx, base.Add(90*time.Second))
	if err != nil {
		t.Fatalf("delete stats: %v", err)
	}
	if affected != 2 {
		t.Fatalf("expected delete 2 stats, got %d", affected)
	}
}

func TestContainerLogsQuery(t *testing.T) {
	s := openTestStorage(t)
	ctx := context.Background()

	base := time.Now().Add(-5 * time.Minute).UTC()
	l1 := ContainerLog{
		ContainerID:   "cid-a",
		ContainerName: "nginx-a",
		Source:        "stdout",
		Level:         "INFO",
		Message:       "ready",
		Timestamp:     base,
	}
	l2 := ContainerLog{
		ContainerID:   "cid-a",
		ContainerName: "nginx-a",
		Source:        "stderr",
		Level:         "ERROR",
		Message:       "Error: something bad happened",
		Timestamp:     base.Add(10 * time.Second),
	}
	if err := s.InsertContainerLog(ctx, &l1); err != nil {
		t.Fatalf("insert l1: %v", err)
	}
	if err := s.InsertContainerLog(ctx, &l2); err != nil {
		t.Fatalf("insert l2: %v", err)
	}

	got, err := s.QueryContainerLogs(ctx, LogQuery{
		ContainerID: "cid-a",
		Contains:    "Error:",
		Limit:       10,
		Desc:        true,
	})
	if err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 log, got %d", len(got))
	}
	if got[0].Level != "ERROR" || got[0].Source != "stderr" {
		t.Fatalf("unexpected log: level=%s source=%s", got[0].Level, got[0].Source)
	}

	affected, err := s.DeleteContainerLogsBefore(ctx, base.Add(5*time.Second))
	if err != nil {
		t.Fatalf("delete logs: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected delete 1 log, got %d", affected)
	}
}

func TestRetentionPruneStatsAndLogs(t *testing.T) {
	s := openTestStorage(t)
	ctx := context.Background()

	now := time.Now().UTC()
	keepAll := 3 * 24 * time.Hour
	keepImportantUntil := 7 * 24 * time.Hour
	cutAll := now.Add(-keepAll)
	cutImportant := now.Add(-keepImportantUntil)

	stats := []ContainerStat{
		{ContainerID: "cid-a", ContainerName: "a", CPUPercent: 1, MemUsageBytes: 1, MemLimitBytes: 1, MemPercent: 1, NetRxBytes: 0, NetTxBytes: 0, BlockReadBytes: 0, BlockWriteBytes: 0, Pids: 1, CollectedAt: now.Add(-8 * 24 * time.Hour)},
		{ContainerID: "cid-a", ContainerName: "a", CPUPercent: 10, MemUsageBytes: 1, MemLimitBytes: 1, MemPercent: 10, NetRxBytes: 0, NetTxBytes: 0, BlockReadBytes: 0, BlockWriteBytes: 0, Pids: 1, CollectedAt: now.Add(-5 * 24 * time.Hour)},
		{ContainerID: "cid-a", ContainerName: "a", CPUPercent: 99, MemUsageBytes: 1, MemLimitBytes: 1, MemPercent: 10, NetRxBytes: 0, NetTxBytes: 0, BlockReadBytes: 0, BlockWriteBytes: 0, Pids: 1, CollectedAt: now.Add(-5 * 24 * time.Hour).Add(1 * time.Minute)},
		{ContainerID: "cid-a", ContainerName: "a", CPUPercent: 5, MemUsageBytes: 1, MemLimitBytes: 1, MemPercent: 5, NetRxBytes: 0, NetTxBytes: 0, BlockReadBytes: 0, BlockWriteBytes: 0, Pids: 1, CollectedAt: now.Add(-1 * 24 * time.Hour)},
	}
	if err := s.InsertContainerStats(ctx, stats); err != nil {
		t.Fatalf("insert stats: %v", err)
	}

	logs := []ContainerLog{
		{ContainerID: "cid-a", ContainerName: "a", Source: "stdout", Level: "INFO", Message: "old", Timestamp: now.Add(-8 * 24 * time.Hour)},
		{ContainerID: "cid-a", ContainerName: "a", Source: "stdout", Level: "INFO", Message: "mid-info", Timestamp: now.Add(-5 * 24 * time.Hour)},
		{ContainerID: "cid-a", ContainerName: "a", Source: "stderr", Level: "INFO", Message: "mid-stderr", Timestamp: now.Add(-5 * 24 * time.Hour).Add(10 * time.Second)},
		{ContainerID: "cid-a", ContainerName: "a", Source: "stdout", Level: "ERROR", Message: "mid-error", Timestamp: now.Add(-5 * 24 * time.Hour).Add(20 * time.Second)},
		{ContainerID: "cid-a", ContainerName: "a", Source: "stdout", Level: "INFO", Message: "new", Timestamp: now.Add(-1 * 24 * time.Hour)},
	}
	if err := s.InsertContainerLogs(ctx, logs); err != nil {
		t.Fatalf("insert logs: %v", err)
	}

	var deleted int64
	for {
		aff, err := s.DeleteContainerStatsBeforeLimited(ctx, cutImportant, 1)
		if err != nil {
			t.Fatalf("delete old stats: %v", err)
		}
		if aff == 0 {
			break
		}
		deleted += aff
	}
	if deleted != 1 {
		t.Fatalf("expected delete 1 old stat, got %d", deleted)
	}

	deleted = 0
	for {
		aff, err := s.DeleteContainerStatsNonAnomalyInRangeLimited(ctx, cutImportant, cutAll, 80, 80, 1)
		if err != nil {
			t.Fatalf("delete mid stats: %v", err)
		}
		if aff == 0 {
			break
		}
		deleted += aff
	}
	if deleted != 1 {
		t.Fatalf("expected delete 1 mid stat, got %d", deleted)
	}

	remainStats, err := s.QueryContainerStats(ctx, StatsQuery{ContainerID: "cid-a", From: &cutImportant, Limit: 50, Desc: false})
	if err != nil {
		t.Fatalf("query remaining stats: %v", err)
	}
	if len(remainStats) != 2 {
		t.Fatalf("expected 2 remaining stats, got %d", len(remainStats))
	}

	var deletedLogs int64
	for {
		aff, err := s.DeleteContainerLogsBeforeLimited(ctx, cutImportant, 2)
		if err != nil {
			t.Fatalf("delete old logs: %v", err)
		}
		if aff == 0 {
			break
		}
		deletedLogs += aff
	}
	if deletedLogs != 1 {
		t.Fatalf("expected delete 1 old log, got %d", deletedLogs)
	}

	deletedLogs = 0
	for {
		aff, err := s.DeleteContainerLogsUnimportantInRangeLimited(ctx, cutImportant, cutAll, []string{"ERROR", "WARN"}, []string{"stderr"}, 1)
		if err != nil {
			t.Fatalf("delete mid logs: %v", err)
		}
		if aff == 0 {
			break
		}
		deletedLogs += aff
	}
	if deletedLogs != 1 {
		t.Fatalf("expected delete 1 mid log, got %d", deletedLogs)
	}

	from := now.Add(-9 * 24 * time.Hour)
	remainLogs, err := s.QueryContainerLogs(ctx, LogQuery{ContainerID: "cid-a", From: &from, Limit: 50, Desc: false})
	if err != nil {
		t.Fatalf("query remaining logs: %v", err)
	}
	if len(remainLogs) != 3 {
		t.Fatalf("expected 3 remaining logs, got %d", len(remainLogs))
	}
}

func TestAuditInsertQueryUpdate(t *testing.T) {
	s := openTestStorage(t)
	ctx := context.Background()

	rec := AuditRecord{
		TraceID:    "trace-1",
		Actor:      "agent",
		Action:     "docker.ps",
		TargetType: "docker",
		TargetID:   "local",
		Status:     "running",
		StartedAt:  time.Now().Add(-1 * time.Second).UTC(),
	}
	if err := s.InsertAuditRecord(ctx, &rec); err != nil {
		t.Fatalf("insert audit: %v", err)
	}
	if rec.ID == 0 {
		t.Fatalf("expected audit id to be set")
	}

	got, err := s.QueryAuditRecords(ctx, AuditQuery{TraceID: "trace-1", Limit: 10})
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(got))
	}
	if got[0].Status != "running" {
		t.Fatalf("unexpected status: %s", got[0].Status)
	}

	status := "success"
	result := `{"ok":true}`
	finished := time.Now().UTC()
	if err := s.UpdateAuditRecord(ctx, rec.ID, AuditUpdate{
		Status:     &status,
		ResultJSON: &result,
		FinishedAt: &finished,
	}); err != nil {
		t.Fatalf("update audit: %v", err)
	}

	got2, err := s.QueryAuditRecords(ctx, AuditQuery{TraceID: "trace-1", Limit: 10})
	if err != nil {
		t.Fatalf("query audit after update: %v", err)
	}
	if len(got2) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(got2))
	}
	if got2[0].Status != "success" || got2[0].ResultJSON != result {
		t.Fatalf("unexpected updated record: status=%s result=%s", got2[0].Status, got2[0].ResultJSON)
	}
}
