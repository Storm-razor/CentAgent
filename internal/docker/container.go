package docker

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// ListContainersOptions 定义 ListContainers 的参数
type ListContainersOptions struct {
	All    bool
	Limit  int
	Status string // running, exited, paused
}

// ContainerSummary 简化版的容器列表信息
type ContainerSummary struct {
	ID      string `json:"id"`
	Names   string `json:"names"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Created int64  `json:"created"`
}

// ListContainers 列出容器
func ListContainers(ctx context.Context, opts ListContainersOptions) ([]ContainerSummary, error) {
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	listOpts := container.ListOptions{
		All:   opts.All,
		Limit: opts.Limit,
	}

	containers, err := cli.ContainerList(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var result []ContainerSummary
	for _, c := range containers {
		// 客户端简单的 Status 过滤
		if opts.Status != "" && c.State != opts.Status {
			continue
		}

		result = append(result, ContainerSummary{
			ID:      truncateID(c.ID),
			Names:   strings.Join(c.Names, ","),
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Created: c.Created,
		})
	}

	return result, nil
}

// ListContainerDetail 列出详细容器
func ListContainerDetail(ctx context.Context, opts ListContainersOptions) ([]ContainerSummary, error) {
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	listOpts := container.ListOptions{
		All:   opts.All,
		Limit: opts.Limit,
	}

	containers, err := cli.ContainerList(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var result []ContainerSummary
	for _, c := range containers {
		if opts.Status != "" && c.State != opts.Status {
			continue
		}

		result = append(result, ContainerSummary{
			ID:      c.ID,
			Names:   strings.Join(c.Names, ","),
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Created: c.Created,
		})
	}

	return result, nil
}

// InspectContainerDetail 简化版的容器详情
type InspectContainerDetail struct {
	ID      string                `json:"id"`
	Name    string                `json:"name"`
	State   *types.ContainerState `json:"state"`
	Image   string                `json:"image"`
	Created string                `json:"created"`
	Config  *container.Config     `json:"config"`
}

// InspectContainer 获取容器详情
func InspectContainer(ctx context.Context, containerID string) (*InspectContainerDetail, error) {
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	jsonRaw, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	return &InspectContainerDetail{
		ID:      jsonRaw.ID,
		Name:    jsonRaw.Name,
		State:   jsonRaw.State,
		Image:   jsonRaw.Config.Image,
		Created: jsonRaw.Created,
		Config:  jsonRaw.Config,
	}, nil
}

// InspectContainerDeatil 获取容器详细详情
func InspectContainerDeatil(ctx context.Context, containerID string) (container.InspectResponse, error) {
	cli, err := GetClient()
	if err != nil {
		return container.InspectResponse{}, err
	}
	return cli.ContainerInspect(ctx, containerID)
}

// StartContainer 启动容器
func StartContainer(ctx context.Context, containerID string) error {
	cli, err := GetClient()
	if err != nil {
		return err
	}
	return cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

// StopContainer 停止容器
func StopContainer(ctx context.Context, containerID string) error {
	cli, err := GetClient()
	if err != nil {
		return err
	}
	// Default timeout (nil means default)
	return cli.ContainerStop(ctx, containerID, container.StopOptions{})
}

// RestartContainer 重启容器
func RestartContainer(ctx context.Context, containerID string) error {
	cli, err := GetClient()
	if err != nil {
		return err
	}
	return cli.ContainerRestart(ctx, containerID, container.StopOptions{})
}

// Events 获取容器事件流
func Events(ctx context.Context, opts events.ListOptions) (<-chan events.Message, <-chan error) {
	cli, err := GetClient()
	if err != nil {
		msgCh := make(chan events.Message)
		errCh := make(chan error, 1)
		close(msgCh)
		errCh <- err
		close(errCh)
		return msgCh, errCh
	}
	return cli.Events(ctx, opts)
}

// GetContainerStats 获取容器统计信息
func GetContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error) {
	cli, err := GetClient()
	if err != nil {
		return container.StatsResponseReader{}, err
	}
	return cli.ContainerStats(ctx, containerID, stream)
}

// GetContainerStatsOneShot 获取容器统计信息（一次）
func GetContainerStatsOneShot(ctx context.Context, containerID string) (container.StatsResponseReader, error) {
	cli, err := GetClient()
	if err != nil {
		return container.StatsResponseReader{}, err
	}
	return cli.ContainerStatsOneShot(ctx, containerID)
}

// RunContainerFromImageOptions 从镜像启动容器的配置项。
type RunContainerFromImageOptions struct {
	// Image 镜像引用（name:tag、digest、或镜像 ID）。
	Image string
	// Name 可选容器名。
	Name string
	// Cmd 可选覆盖镜像默认 CMD。
	Cmd []string
	// Env 容器环境变量（形如 KEY=VALUE）。
	Env []string
	// WorkingDir 可选工作目录。
	WorkingDir string
	// Labels 容器标签。
	Labels map[string]string
	// AutoRemove 容器退出后是否自动删除。
	AutoRemove bool
	// RestartPolicy 重启策略（no/always/unless-stopped/on-failure）。
	RestartPolicy string
	// Binds 挂载配置，语法与 docker CLI -v 一致（支持卷或宿主机路径）。
	// 例如：myvol:/data 或 /host/path:/data:ro
	Binds []string
	// Network 可选网络名/ID（创建时加入该网络）。
	Network string
	// Publish 端口映射规则列表，语法类似 docker CLI -p。
	// 支持：hostPort:containerPort、hostIP:hostPort:containerPort，
	// 并可在 containerPort 上附带协议：containerPort/tcp 或 containerPort/udp。
	Publish []string
	// PullIfMissing 若本地不存在镜像，是否尝试拉取。
	PullIfMissing bool
}

// RunContainerResult 启动容器的结果（用于对外输出）。
type RunContainerResult struct {
	// ContainerID 启动后的容器 ID。
	ContainerID string `json:"container_id"`
	// Name 容器名（含前导 /）。
	Name string `json:"name"`
	// Warnings Docker 可能返回的警告信息。
	Warnings []string `json:"warnings,omitempty"`
}

// RunContainerFromImage 从镜像创建并启动一个容器。
func RunContainerFromImage(ctx context.Context, opts RunContainerFromImageOptions) (*RunContainerResult, error) {
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	imageRef := strings.TrimSpace(opts.Image)
	if imageRef == "" {
		return nil, fmt.Errorf("image is required")
	}

	if opts.PullIfMissing {
		if _, _, err := cli.ImageInspectWithRaw(ctx, imageRef); err != nil {
			reader, pullErr := cli.ImagePull(ctx, imageRef, image.PullOptions{})
			if pullErr != nil {
				return nil, fmt.Errorf("failed to pull image %s: %w", imageRef, pullErr)
			}
			_, _ = io.Copy(io.Discard, reader)
			_ = reader.Close()
		}
	}

	exposed := nat.PortSet{}
	portBindings := nat.PortMap{}
	for _, spec := range opts.Publish {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}

		hostIP, hostPort, contPart, err := parsePublishSpec(spec)
		if err != nil {
			return nil, err
		}
		containerPort, proto := splitContainerPortProto(contPart)

		p, err := nat.NewPort(proto, containerPort)
		if err != nil {
			return nil, fmt.Errorf("invalid publish spec %q: %w", spec, err)
		}
		exposed[p] = struct{}{}
		portBindings[p] = append(portBindings[p], nat.PortBinding{
			HostIP:   hostIP,
			HostPort: hostPort,
		})
	}

	cfg := &container.Config{
		Image:        imageRef,
		Cmd:          opts.Cmd,
		Env:          opts.Env,
		WorkingDir:   opts.WorkingDir,
		Labels:       opts.Labels,
		ExposedPorts: exposed,
	}

	hostCfg := &container.HostConfig{
		AutoRemove:   opts.AutoRemove,
		Binds:        opts.Binds,
		PortBindings: portBindings,
	}
	if strings.TrimSpace(opts.RestartPolicy) != "" {
		hostCfg.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyMode(strings.TrimSpace(opts.RestartPolicy))}
	}

	netCfg := &network.NetworkingConfig{}
	if strings.TrimSpace(opts.Network) != "" {
		netCfg.EndpointsConfig = map[string]*network.EndpointSettings{
			strings.TrimSpace(opts.Network): {},
		}
	}

	resp, err := cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, strings.TrimSpace(opts.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to create container from image %s: %w", imageRef, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container %s: %w", resp.ID, err)
	}

	inspected, err := cli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return &RunContainerResult{
			ContainerID: resp.ID,
			Warnings:    resp.Warnings,
		}, nil
	}

	return &RunContainerResult{
		ContainerID: resp.ID,
		Name:        inspected.Name,
		Warnings:    resp.Warnings,
	}, nil
}

func parsePublishSpec(spec string) (hostIP string, hostPort string, contPart string, err error) {
	parts := strings.Split(spec, ":")
	switch len(parts) {
	case 2:
		return "", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
	case 3:
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2]), nil
	default:
		return "", "", "", fmt.Errorf("invalid publish spec %q", spec)
	}
}

func splitContainerPortProto(contPart string) (containerPort string, proto string) {
	containerPort = strings.TrimSpace(contPart)
	proto = "tcp"
	if p, protoRaw, ok := strings.Cut(containerPort, "/"); ok {
		containerPort = strings.TrimSpace(p)
		if strings.TrimSpace(protoRaw) != "" {
			proto = strings.TrimSpace(protoRaw)
		}
	}
	return containerPort, proto
}
