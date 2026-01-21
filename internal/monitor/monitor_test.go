package monitor

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/wwwzy/CentAgent/internal/docker"
	"github.com/wwwzy/CentAgent/internal/storage"
)

func openTestStorage(t *testing.T, ctx context.Context) *storage.Storage {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "centagent-test.db")
	store, err := storage.Open(ctx, storage.Config{Path: dbPath, EnableWAL: true})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func requireDocker(t *testing.T) {
	t.Helper()

	cli, err := docker.GetClient()
	if err != nil {
		t.Skipf("docker client unavailable: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := cli.Ping(ctx); err != nil {
		t.Skipf("docker daemon unavailable: %v", err)
	}
}

func ensureAnyRunningContainer(t *testing.T, ctx context.Context) string {
	t.Helper()

	containers, err := docker.ListContainers(ctx, docker.ListContainersOptions{All: false, Status: "running"})
	if err == nil && len(containers) > 0 {
		return containers[0].ID
	}

	cli, err := docker.GetClient()
	if err != nil {
		t.Fatalf("get docker client: %v", err)
	}

	imageName := "nginx:alpine"
	if _, err := cli.ImageInspect(ctx, imageName); err != nil {
		if errdefs.IsNotFound(err) {
			reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
			if err != nil {
				t.Skipf("pull image %s failed: %v", imageName, err)
			}
			defer reader.Close()
			_, _ = io.Copy(io.Discard, reader)
		} else {
			t.Skipf("inspect image %s failed: %v", imageName, err)
		}
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{Image: imageName},
		&container.HostConfig{AutoRemove: true},
		&network.NetworkingConfig{},
		&v1.Platform{},
		"centagent-monitor-stats-test",
	)
	if err != nil {
		t.Skipf("create container failed: %v", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		t.Skipf("start container failed: %v", err)
	}

	t.Cleanup(func() {
		_ = cli.ContainerStop(context.Background(), resp.ID, container.StopOptions{})
	})

	return resp.ID[:12]
}

func TestManager_StatsPipeline_WritesRealDockerStatsToStorage(t *testing.T) {
	requireDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	store := openTestStorage(t, ctx)

	targetID := ensureAnyRunningContainer(t, ctx)

	statsCollector, err := NewStatsCollector(store)
	if err != nil {
		t.Fatalf("new stats collector: %v", err)
	}
	retCollector, err := NewRetentionCollector(store)
	if err != nil {
		t.Fatalf("new retention collector: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Stats.Interval = 200 * time.Millisecond
	cfg.Stats.FlushInterval = 50 * time.Millisecond
	cfg.Stats.BatchSize = 50
	cfg.Stats.Workers = 2
	cfg.Stats.QueueSize = 64

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	mgr.WithStats(statsCollector)
	mgr.WithRetention(retCollector)

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer mgr.Stop()

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		got, err := store.QueryContainerStats(ctx, storage.StatsQuery{ContainerID: targetID, Limit: 10, Desc: true})
		if err != nil {
			t.Fatalf("query container stats: %v", err)
		}
		if len(got) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	got, err := store.QueryContainerStats(ctx, storage.StatsQuery{ContainerID: targetID, Limit: 10, Desc: true})
	if err != nil {
		t.Fatalf("query container stats: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("expected stats to be written for container %s", targetID)
	}

	latest := got[0]
	if latest.ContainerID == "" || latest.CollectedAt.IsZero() {
		t.Fatalf("expected non-empty ContainerID and CollectedAt")
	}

	t.Logf("latest stats: %+v", latest)
}

func TestManager_LogPipeline_CollectsRealDockerLogsToStorage(t *testing.T) {
	requireDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	store := openTestStorage(t, ctx)

	statsCollector, err := NewStatsCollector(store)
	if err != nil {
		t.Fatalf("new stats collector: %v", err)
	}
	logCollector, err := NewLogCollector(store)
	if err != nil {
		t.Fatalf("new log collector: %v", err)
	}
	retCollector, err := NewRetentionCollector(store)
	if err != nil {
		t.Fatalf("new retention collector: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Stats.Enabled = false
	cfg.Logs.Enabled = true
	cfg.Logs.QueueSize = 256
	cfg.Logs.BatchSize = 50
	cfg.Logs.FlushInterval = 100 * time.Millisecond
	cfg.Logs.ReconnectDelay = 300 * time.Millisecond
	cfg.Logs.ReconnectJitter = 50 * time.Millisecond

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	mgr.WithStats(statsCollector)
	mgr.WithLogs(logCollector)
	mgr.WithRetention(retCollector)

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer mgr.Stop()

	time.Sleep(300 * time.Millisecond)

	cli, err := docker.GetClient()
	if err != nil {
		t.Fatalf("get docker client: %v", err)
	}

	imageName := "docker.1ms.run/library/alpine"
	if _, err := cli.ImageInspect(ctx, imageName); err != nil {
		if errdefs.IsNotFound(err) {
			reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
			if err != nil {
				t.Skipf("pull image %s failed: %v", imageName, err)
			}
			defer reader.Close()
			_, _ = io.Copy(io.Discard, reader)
		} else {
			t.Skipf("inspect image %s failed: %v", imageName, err)
		}
	}

	containerName := fmt.Sprintf("centagent-monitor-logs-test-%d", time.Now().UnixNano())
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageName,
			Cmd:   []string{"sh", "-c", "i=0; while [ $i -lt 30 ]; do echo Error: monitor-log-test-$i; i=$((i+1)); sleep 0.1; done; sleep 1"},
		},
		&container.HostConfig{
			AutoRemove: true,
		},
		&network.NetworkingConfig{},
		&v1.Platform{},
		containerName,
	)
	if err != nil {
		t.Skipf("create container failed: %v", err)
	}
	containerID := resp.ID

	t.Cleanup(func() {
		_ = cli.ContainerStop(context.Background(), containerID, container.StopOptions{})
	})

	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		_ = cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		t.Skipf("start container failed: %v", err)
	}

	waitCh, waitErrCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case <-ctx.Done():
		t.Fatalf("wait container exit: %v", ctx.Err())
	case err := <-waitErrCh:
		if err != nil {
			t.Fatalf("wait container exit: %v", err)
		}
	case <-waitCh:
	}

	expectedLines := 30
	deadline := time.Now().Add(8 * time.Second)
	var all []storage.ContainerLog
	for time.Now().Before(deadline) {
		got, err := store.QueryContainerLogs(ctx, storage.LogQuery{
			ContainerID: containerID,
			Contains:    "Error: monitor-log-test-",
			Limit:       5000,
			Desc:        false,
		})
		if err != nil {
			t.Fatalf("query container logs: %v", err)
		}
		all = got
		if len(all) >= expectedLines {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if len(all) == 0 {
		t.Fatalf("expected logs to be collected for container %s", containerID)
	}

	t.Logf("all logs (%d): %+v", len(all), all)
	if len(all) < expectedLines {
		t.Fatalf("expected at least %d logs, got %d", expectedLines, len(all))
	}
}

func TestRetentionCollector_RunOnce_PrunesByPolicy(t *testing.T) {
	ctx := context.Background()
	store := openTestStorage(t, ctx)

	now := time.Now().UTC()

	stats := []storage.ContainerStat{
		{ContainerID: "cid-a", ContainerName: "a", CPUPercent: 1, MemUsageBytes: 1, MemLimitBytes: 1, MemPercent: 1, NetRxBytes: 0, NetTxBytes: 0, BlockReadBytes: 0, BlockWriteBytes: 0, Pids: 1, CollectedAt: now.Add(-8 * 24 * time.Hour)},
		{ContainerID: "cid-a", ContainerName: "a", CPUPercent: 10, MemUsageBytes: 1, MemLimitBytes: 1, MemPercent: 10, NetRxBytes: 0, NetTxBytes: 0, BlockReadBytes: 0, BlockWriteBytes: 0, Pids: 1, CollectedAt: now.Add(-5 * 24 * time.Hour)},
		{ContainerID: "cid-a", ContainerName: "a", CPUPercent: 99, MemUsageBytes: 1, MemLimitBytes: 1, MemPercent: 10, NetRxBytes: 0, NetTxBytes: 0, BlockReadBytes: 0, BlockWriteBytes: 0, Pids: 1, CollectedAt: now.Add(-5 * 24 * time.Hour).Add(1 * time.Minute)},
		{ContainerID: "cid-a", ContainerName: "a", CPUPercent: 5, MemUsageBytes: 1, MemLimitBytes: 1, MemPercent: 5, NetRxBytes: 0, NetTxBytes: 0, BlockReadBytes: 0, BlockWriteBytes: 0, Pids: 1, CollectedAt: now.Add(-1 * 24 * time.Hour)},
	}
	if err := store.InsertContainerStats(ctx, stats); err != nil {
		t.Fatalf("insert stats: %v", err)
	}

	logs := []storage.ContainerLog{
		{ContainerID: "cid-a", ContainerName: "a", Source: "stdout", Level: "INFO", Message: "old", Timestamp: now.Add(-8 * 24 * time.Hour)},
		{ContainerID: "cid-a", ContainerName: "a", Source: "stdout", Level: "INFO", Message: "mid-info", Timestamp: now.Add(-5 * 24 * time.Hour)},
		{ContainerID: "cid-a", ContainerName: "a", Source: "stderr", Level: "INFO", Message: "mid-stderr", Timestamp: now.Add(-5 * 24 * time.Hour).Add(10 * time.Second)},
		{ContainerID: "cid-a", ContainerName: "a", Source: "stdout", Level: "ERROR", Message: "mid-error", Timestamp: now.Add(-5 * 24 * time.Hour).Add(20 * time.Second)},
		{ContainerID: "cid-a", ContainerName: "a", Source: "stdout", Level: "INFO", Message: "new", Timestamp: now.Add(-1 * 24 * time.Hour)},
	}
	if err := store.InsertContainerLogs(ctx, logs); err != nil {
		t.Fatalf("insert logs: %v", err)
	}

	ret, err := NewRetentionCollector(store)
	if err != nil {
		t.Fatalf("new retention collector: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Stats.Enabled = false
	cfg.Logs.Enabled = false
	cfg.Retention.Enabled = true
	cfg.Retention.Interval = 24 * time.Hour
	cfg.Retention.Workers = 2
	cfg.Retention.BatchRows = 1
	cfg.Retention.IdleSleep = 0
	cfg.Retention.Stats.KeepAll = 3 * 24 * time.Hour
	cfg.Retention.Stats.KeepAnomalyUntil = 7 * 24 * time.Hour
	cfg.Retention.Stats.CPUHigh = 80
	cfg.Retention.Stats.MemHigh = 80
	cfg.Retention.Logs.KeepAll = 3 * 24 * time.Hour
	cfg.Retention.Logs.KeepImportantUntil = 7 * 24 * time.Hour
	cfg.Retention.Logs.KeepLevels = []string{"ERROR", "WARN"}
	cfg.Retention.Logs.KeepSources = []string{"stderr"}

	mgrCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	mgr.WithRetention(ret)
	if err := mgr.Start(mgrCtx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer func() {
		mgr.Stop()
		_ = mgr.Wait()
	}()

	fromStats := now.Add(-10 * 24 * time.Hour)
	fromLogs := now.Add(-10 * 24 * time.Hour)
	deadline := time.Now().Add(1500 * time.Millisecond)

	for time.Now().Before(deadline) {
		remainStats, err := store.QueryContainerStats(ctx, storage.StatsQuery{ContainerID: "cid-a", From: &fromStats, Limit: 50, Desc: false})
		if err != nil {
			t.Fatalf("query remaining stats: %v", err)
		}
		remainLogs, err := store.QueryContainerLogs(ctx, storage.LogQuery{ContainerID: "cid-a", From: &fromLogs, Limit: 50, Desc: false})
		if err != nil {
			t.Fatalf("query remaining logs: %v", err)
		}
		if len(remainStats) == 2 && len(remainLogs) == 3 {
			t.Logf("remaining stats: %+v", remainStats)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	remainStats, err := store.QueryContainerStats(ctx, storage.StatsQuery{ContainerID: "cid-a", From: &fromStats, Limit: 50, Desc: false})
	if err != nil {
		t.Fatalf("query remaining stats: %v", err)
	}
	remainLogs, err := store.QueryContainerLogs(ctx, storage.LogQuery{ContainerID: "cid-a", From: &fromLogs, Limit: 50, Desc: false})
	if err != nil {
		t.Fatalf("query remaining logs: %v", err)
	}
	if len(remainStats) != 2 || len(remainLogs) != 3 {
		t.Fatalf("unexpected remaining rows: stats=%d logs=%d", len(remainStats), len(remainLogs))
	}
}
