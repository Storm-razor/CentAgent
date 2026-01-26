package docker

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func requireDocker(t *testing.T) {
	t.Helper()

	cli, err := GetClient()
	if err != nil {
		t.Skipf("docker client unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		t.Skipf("docker daemon unavailable: %v", err)
	}
}

// setupTestContainer 启动一个测试用的容器 (nginx:alpine)，如果本地没有镜像会自动拉取
// 返回容器ID和清理函数
func setupTestContainer(t *testing.T, ctx context.Context) (string, func()) {
	requireDocker(t)

	cli, err := GetClient()
	if err != nil {
		t.Skipf("Failed to get docker client: %v", err)
	}

	// 尝试优先使用本地镜像
	var imageName string
	images, err := cli.ImageList(ctx, image.ListOptions{All: false})
	if err == nil && len(images) > 0 {
		for _, img := range images {
			if len(img.RepoTags) > 0 && img.RepoTags[0] != "<none>:<none>" {
				tag := img.RepoTags[0]
				if strings.Contains(tag, "alpine") || strings.Contains(tag, "busybox") {
					imageName = tag
					t.Logf("Using local image: %s", imageName)
					break
				}
				continue
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

	cfg := &container.Config{Image: imageName}
	if !strings.Contains(imageName, "nginx") {
		cfg.Cmd = []string{"sh", "-c", "sleep 600"}
	}

	resp, err := cli.ContainerCreate(ctx,
		cfg,
		&container.HostConfig{
			AutoRemove: false,
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
		if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
			t.Logf("Failed to stop container %s: %v", containerID, err)
		}
		if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
			t.Logf("Failed to remove container %s: %v", containerID, err)
		}
		// 稍微等待一下
		time.Sleep(1 * time.Second)
	}

	return containerID, cleanup
}

func TestGetClient(t *testing.T) {
	requireDocker(t)

	cli, err := GetClient()
	if err != nil {
		t.Skipf("GetClient failed: %v", err)
	}
	if cli == nil {
		t.Fatal("GetClient returned nil client")
	}

	// 验证是否能 Ping 通
	ctx := context.Background()
	ping, err := cli.Ping(ctx)
	if err != nil {
		t.Skipf("Failed to ping docker daemon: %v", err)
	}
	t.Logf("Docker Daemon API Version: %s", ping.APIVersion)
}

func TestListContainers(t *testing.T) {
	requireDocker(t)

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
	requireDocker(t)

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
	requireDocker(t)

	ctx := context.Background()
	containerID, cleanup := setupTestContainer(t, ctx)
	defer cleanup()

	// 1. Restart
	t.Log("Testing RestartContainer...")
	if err := RestartContainer(ctx, containerID); err != nil {
		t.Fatalf("RestartContainer failed: %v", err)
	}

	// 等待一下状态变化
	time.Sleep(2 * time.Second)

	info, err := InspectContainer(ctx, containerID)
	if err != nil {
		t.Fatalf("InspectContainer after restart failed: %v", err)
	}
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
	if info != nil && info.State.Status == "running" {
		t.Errorf("Container should not be running after stop")
	}
}

func TestGetContainerLogs(t *testing.T) {
	requireDocker(t)

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

func TestListImages(t *testing.T) {
	requireDocker(t)

	ctx := context.Background()
	images, err := ListImages(ctx, ListImagesOptions{All: false})
	if err != nil {
		t.Fatalf("ListImages failed: %v", err)
	}
	t.Logf("Found %d images", len(images))
}

func TestInspectImage(t *testing.T) {
	requireDocker(t)

	ctx := context.Background()
	images, err := ListImages(ctx, ListImagesOptions{All: false})
	if err != nil {
		t.Fatalf("ListImages failed: %v", err)
	}
	if len(images) == 0 {
		t.Skip("no local images to inspect")
	}

	var ref string
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag != "" && tag != "<none>:<none>" {
				ref = tag
				break
			}
		}
		if ref != "" {
			break
		}
	}
	if ref == "" {
		t.Skip("no tagged image to inspect")
	}

	info, err := InspectImage(ctx, ref)
	if err != nil {
		t.Fatalf("InspectImage failed: %v", err)
	}
	if info.ID == "" {
		t.Fatalf("InspectImage returned empty ID for %s", ref)
	}
	t.Logf("Inspected image %s: id=%s size=%d", ref, info.ID, info.Size)
}

func TestVolumeLifecycle(t *testing.T) {
	requireDocker(t)

	ctx := context.Background()
	name := fmt.Sprintf("centagent-test-vol-%d", time.Now().UnixNano())

	created, err := CreateVolume(ctx, CreateVolumeOptions{
		Name:   name,
		Driver: "local",
		Labels: map[string]string{"centagent_test": "true"},
	})
	if err != nil {
		t.Fatalf("CreateVolume failed: %v", err)
	}
	if created.Name != name {
		t.Fatalf("CreateVolume expected name %s, got %s", name, created.Name)
	}

	inspected, err := InspectVolume(ctx, name)
	if err != nil {
		_ = RemoveVolume(ctx, name, RemoveVolumeOptions{Force: true})
		t.Fatalf("InspectVolume failed: %v", err)
	}
	if inspected.Name != name {
		_ = RemoveVolume(ctx, name, RemoveVolumeOptions{Force: true})
		t.Fatalf("InspectVolume expected name %s, got %s", name, inspected.Name)
	}

	if inspected.Labels["centagent_test"] != "true" {
		_ = RemoveVolume(ctx, name, RemoveVolumeOptions{Force: true})
		t.Fatalf("InspectVolume expected label centagent_test=true, got %v", inspected.Labels)
	}

	if err := RemoveVolume(ctx, name, RemoveVolumeOptions{Force: true}); err != nil {
		t.Fatalf("RemoveVolume failed: %v", err)
	}
}

func TestNetworkLifecycle(t *testing.T) {
	requireDocker(t)

	ctx := context.Background()
	name := fmt.Sprintf("centagent-test-net-%d", time.Now().UnixNano())

	created, err := CreateNetwork(ctx, CreateNetworkOptions{
		Name:       name,
		Driver:     "bridge",
		Attachable: true,
		Labels:     map[string]string{"centagent_test": "true"},
	})
	if err != nil {
		t.Fatalf("CreateNetwork failed: %v", err)
	}

	info, err := InspectNetwork(ctx, created.ID)
	if err != nil {
		_ = RemoveNetwork(ctx, created.ID)
		t.Fatalf("InspectNetwork failed: %v", err)
	}
	if info.Name != name {
		_ = RemoveNetwork(ctx, created.ID)
		t.Fatalf("InspectNetwork expected name %s, got %s", name, info.Name)
	}

	if err := RemoveNetwork(ctx, created.ID); err != nil {
		t.Fatalf("RemoveNetwork failed: %v", err)
	}
}

func TestRunContainerFromImage(t *testing.T) {
	requireDocker(t)

	ctx := context.Background()
	cli, err := GetClient()
	if err != nil {
		t.Skipf("Failed to get docker client: %v", err)
	}

	images, err := cli.ImageList(ctx, image.ListOptions{All: false})
	if err != nil || len(images) == 0 {
		t.Skip("no local images to run a container")
	}

	var imageName string
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == "" || tag == "<none>:<none>" {
				continue
			}
			if strings.Contains(tag, "alpine") || strings.Contains(tag, "busybox") {
				imageName = tag
				break
			}
		}
		if imageName != "" && strings.Contains(imageName, "nginx") {
			break
		}
	}
	if imageName == "" {
		t.Skip("no suitable tagged image found to run a container")
	}

	name := fmt.Sprintf("centagent-run-%d", time.Now().UnixNano())
	runOpts := RunContainerFromImageOptions{
		Image:         imageName,
		Name:          name,
		AutoRemove:    false,
		PullIfMissing: false,
	}
	if !strings.Contains(imageName, "nginx") {
		runOpts.Cmd = []string{"sh", "-c", "sleep 600"}
	}

	res, err := RunContainerFromImage(ctx, runOpts)
	if err != nil {
		t.Fatalf("RunContainerFromImage failed: %v", err)
	}
	if res == nil || res.ContainerID == "" {
		t.Fatalf("RunContainerFromImage returned empty container id: %+v", res)
	}
	t.Logf("Started container id=%s name=%s", res.ContainerID, res.Name)

	defer func() {
		_ = cli.ContainerStop(ctx, res.ContainerID, container.StopOptions{})
		_ = cli.ContainerRemove(ctx, res.ContainerID, container.RemoveOptions{Force: true})
	}()

	info, err := InspectContainer(ctx, res.ContainerID)
	if err != nil {
		t.Fatalf("InspectContainer failed: %v", err)
	}
	if info.State == nil || info.State.Status != "running" {
		t.Fatalf("expected container running, got state=%v", info.State)
	}
}
