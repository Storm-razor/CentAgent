package monitor

import (
	"context"
	"io"
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
	t.Parallel()
	requireDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	store, err := storage.Open(ctx, storage.Config{InMemory: true})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer func() { _ = store.Close() }()

	targetID := ensureAnyRunningContainer(t, ctx)

	statsCollector, err := NewStatsCollector(store)
	if err != nil {
		t.Fatalf("new stats collector: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Stats.Interval = 200 * time.Millisecond
	cfg.Stats.FlushInterval = 50 * time.Millisecond
	cfg.Stats.BatchSize = 50
	cfg.Stats.Workers = 2
	cfg.Stats.QueueSize = 64

	mgr, err := NewManager(cfg, statsCollector)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

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
