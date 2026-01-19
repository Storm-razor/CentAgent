package docker

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// setupTestContainer 启动一个测试用的容器 (nginx:alpine)，如果本地没有镜像会自动拉取
// 返回容器ID和清理函数
func setupTestContainer(t *testing.T, ctx context.Context) (string, func()) {
	cli, err := GetClient()
	if err != nil {
		t.Fatalf("Failed to get docker client: %v", err)
	}

	// 尝试优先使用本地镜像
	var imageName string
	images, err := cli.ImageList(ctx, image.ListOptions{All: false})
	if err == nil && len(images) > 0 {
		for _, img := range images {
			if len(img.RepoTags) > 0 && img.RepoTags[0] != "<none>:<none>" {
				imageName = img.RepoTags[0]
				t.Logf("Using local image: %s", imageName)
				break
			}
		}
	}

	if imageName == "" {
		imageName = "nginx:alpine"
		t.Logf("No local image found, trying to pull %s...", imageName)
		// 1. 检查并拉取镜像
		_, err = cli.ImageInspect(ctx, imageName)
		if err != nil {
			if errdefs.IsNotFound(err) {
				t.Logf("Image %s not found, pulling...", imageName)
				reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
				if err != nil {
					t.Skipf("Failed to pull image %s (network issue?): %v. Skipping test.", imageName, err)
				}
				defer reader.Close()
				// 等待拉取完成
				io.Copy(io.Discard, reader)
			} else {
				t.Fatalf("Failed to inspect image %s: %v", imageName, err)
			}
		}
	}

	// 2. 创建容器
	containerName := fmt.Sprintf("centagent-test-%d", time.Now().UnixNano())
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageName,
		},
		&container.HostConfig{
			AutoRemove: true, // 测试完自动删除
		},
		&network.NetworkingConfig{},
		&v1.Platform{},
		containerName,
	)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	containerID := resp.ID
	t.Logf("Created test container: %s (%s)", containerName, containerID)

	// 3. 启动容器
	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		// 尝试清理
		_ = cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		t.Fatalf("Failed to start container: %v", err)
	}

	// 返回清理函数
	cleanup := func() {
		t.Logf("Cleaning up container %s...", containerID)
		// 这里虽然设置了 AutoRemove，但为了保险还是显式 Stop 一下
		// 显式 Stop 后，AutoRemove 会起作用自动删除
		if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
			t.Logf("Failed to stop container %s: %v", containerID, err)
		}
		// 稍微等待一下
		time.Sleep(1 * time.Second)
	}

	return containerID, cleanup
}

func TestGetClient(t *testing.T) {
	cli, err := GetClient()
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
	if cli == nil {
		t.Fatal("GetClient returned nil client")
	}

	// 验证是否能 Ping 通
	ctx := context.Background()
	ping, err := cli.Ping(ctx)
	if err != nil {
		t.Fatalf("Failed to ping docker daemon: %v. Make sure Docker Desktop is running.", err)
	}
	t.Logf("Docker Daemon API Version: %s", ping.APIVersion)
}

func TestListContainers(t *testing.T) {
	ctx := context.Background()
	// 先启动一个测试容器确保列表不为空
	_, cleanup := setupTestContainer(t, ctx)
	defer cleanup()

	// 测试列出所有容器
	containers, err := ListContainers(ctx, ListContainersOptions{All: true})
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}

	if len(containers) == 0 {
		t.Error("Expected at least one container, got 0")
	}

	found := false
	for _, c := range containers {
		t.Logf("Found container: %s (Status: %s)", c.Names, c.Status)
		if c.State == "running" {
			found = true
		}
	}
	if !found {
		t.Log("Warning: No running containers found, but we just started one?")
	}
}

func TestInspectContainer(t *testing.T) {
	ctx := context.Background()
	containerID, cleanup := setupTestContainer(t, ctx)
	defer cleanup()

	info, err := InspectContainer(ctx, containerID)
	if err != nil {
		t.Fatalf("InspectContainer failed: %v", err)
	}

	if info.ID != containerID {
		t.Errorf("Expected ID %s, got %s", containerID, info.ID)
	}
	t.Log("Inspected container image", info)
}

func TestContainerLifecycle(t *testing.T) {
	ctx := context.Background()
	containerID, cleanup := setupTestContainer(t, ctx)
	defer cleanup()

	// 1. Restart
	t.Log("Testing RestartContainer...")
	if err := RestartContainer(ctx, containerID); err != nil {
		t.Errorf("RestartContainer failed: %v", err)
	}

	// 等待一下状态变化
	time.Sleep(2 * time.Second)

	info, _ := InspectContainer(ctx, containerID)
	if info.State.Status != "running" {
		t.Errorf("Container should be running after restart, got %s", info.State.Status)
	}

	// 2. Stop
	t.Log("Testing StopContainer...")
	if err := StopContainer(ctx, containerID); err != nil {
		t.Errorf("StopContainer failed: %v", err)
	}

	// 再次检查状态
	info, _ = InspectContainer(ctx, containerID)
	// AutoRemove=true 的容器 Stop 后会消失，Inspect 可能会报错 NotFound，或者状态是 exited
	// 如果 Inspect 报错 NotFound，说明 AutoRemove 生效了，这也是一种成功
	if info != nil && info.State.Status == "running" {
		t.Errorf("Container should not be running after stop")
	}
}

func TestGetContainerLogs(t *testing.T) {
	ctx := context.Background()
	containerID, cleanup := setupTestContainer(t, ctx)
	defer cleanup()

	// 稍等容器产生日志 (nginx 启动通常会有日志)
	time.Sleep(2 * time.Second)

	logs, err := GetContainerLogs(ctx, GetContainerLogsOptions{
		ContainerID: containerID,
		Tail:        "10",
	})
	if err != nil {
		t.Fatalf("GetContainerLogs failed: %v", err)
	}

	if len(logs) == 0 {	
		t.Log("Warning: Logs are empty")
	} else {
		// 为了调试方便，在测试中我们可以打印更长的日志，或者干脆全部打印
		// 之前这里限制了只打印前 100 个字符：logs[:min(len(logs), 100)]
		// 现在我们改为打印全部（或者一个较大的长度，如 2000）
		t.Logf("Got logs (len=%d):\n%s", len(logs), logs)
	}
}
