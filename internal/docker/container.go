package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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
			ID:      c.ID[:12],
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
