package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
)

type ListNetworksOptions struct {
	// Filters 列表过滤条件，key/value 语义与 Docker Engine API 一致。
	Filters map[string][]string
}

// NetworkSummary 网络列表的简化信息（用于 list 输出）。
type NetworkSummary struct {
	// ID 网络 ID（列表场景会做截断便于阅读）。
	ID string `json:"id"`
	// Name 网络名称。
	Name string `json:"name"`
	// Driver 网络驱动（如 bridge、overlay）。
	Driver string `json:"driver"`
	// Scope 网络作用域（如 local、swarm）。
	Scope string `json:"scope"`
}

func ListNetworks(ctx context.Context, opts ListNetworksOptions) ([]NetworkSummary, error) {
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	listOpts := network.ListOptions{
		Filters: filters.NewArgs(),
	}
	for k, vs := range opts.Filters {
		for _, v := range vs {
			listOpts.Filters.Add(k, v)
		}
	}

	networks, err := cli.NetworkList(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	result := make([]NetworkSummary, 0, len(networks))
	for _, n := range networks {
		result = append(result, NetworkSummary{
			ID:     truncateID(n.ID),
			Name:   n.Name,
			Driver: n.Driver,
			Scope:  n.Scope,
		})
	}
	return result, nil
}

type CreateNetworkOptions struct {
	// Name 网络名称。
	Name string
	// Driver 网络驱动（如 bridge、overlay），空值代表使用 Docker 默认值。
	Driver string
	// Internal 是否为内部网络（限制外部访问）。
	Internal bool
	// Attachable 是否允许手动将容器 attach 到该网络。
	Attachable bool
	// Labels 网络标签。
	Labels map[string]string
	// Options 驱动相关的额外选项。
	Options map[string]string
	// IPAM IP 地址管理配置（子网、网关等）。
	IPAM *network.IPAM
}

func CreateNetwork(ctx context.Context, opts CreateNetworkOptions) (network.CreateResponse, error) {
	cli, err := GetClient()
	if err != nil {
		return network.CreateResponse{}, err
	}

	resp, err := cli.NetworkCreate(ctx, opts.Name, network.CreateOptions{
		Driver:     opts.Driver,
		Internal:   opts.Internal,
		Attachable: opts.Attachable,
		Labels:     opts.Labels,
		Options:    opts.Options,
		IPAM:       opts.IPAM,
	})
	if err != nil {
		return network.CreateResponse{}, fmt.Errorf("failed to create network %s: %w", opts.Name, err)
	}
	return resp, nil
}

func InspectNetwork(ctx context.Context, networkID string) (network.Inspect, error) {
	cli, err := GetClient()
	if err != nil {
		return network.Inspect{}, err
	}
	inspected, err := cli.NetworkInspect(ctx, networkID, network.InspectOptions{})
	if err != nil {
		return network.Inspect{}, fmt.Errorf("failed to inspect network %s: %w", networkID, err)
	}
	return inspected, nil
}

type ConnectNetworkOptions struct {
	// ContainerID 要连接到网络的容器 ID 或名称。
	ContainerID string
	// EndpointConfig 可选的 endpoint 配置（别名、固定 IP 等）。
	EndpointConfig *network.EndpointSettings
}

func ConnectNetwork(ctx context.Context, networkID string, opts ConnectNetworkOptions) error {
	cli, err := GetClient()
	if err != nil {
		return err
	}
	if err := cli.NetworkConnect(ctx, networkID, opts.ContainerID, opts.EndpointConfig); err != nil {
		return fmt.Errorf("failed to connect container %s to network %s: %w", opts.ContainerID, networkID, err)
	}
	return nil
}

type DisconnectNetworkOptions struct {
	// ContainerID 要从网络断开的容器 ID 或名称。
	ContainerID string
	// Force 是否强制断开。
	Force bool
}

func DisconnectNetwork(ctx context.Context, networkID string, opts DisconnectNetworkOptions) error {
	cli, err := GetClient()
	if err != nil {
		return err
	}
	if err := cli.NetworkDisconnect(ctx, networkID, opts.ContainerID, opts.Force); err != nil {
		return fmt.Errorf("failed to disconnect container %s from network %s: %w", opts.ContainerID, networkID, err)
	}
	return nil
}

func RemoveNetwork(ctx context.Context, networkID string) error {
	cli, err := GetClient()
	if err != nil {
		return err
	}
	if err := cli.NetworkRemove(ctx, networkID); err != nil {
		return fmt.Errorf("failed to remove network %s: %w", networkID, err)
	}
	return nil
}

func PruneNetworks(ctx context.Context, filterMap map[string][]string) (network.PruneReport, error) {
	cli, err := GetClient()
	if err != nil {
		return network.PruneReport{}, err
	}
	f := filters.NewArgs()
	for k, vs := range filterMap {
		for _, v := range vs {
			f.Add(k, v)
		}
	}
	report, err := cli.NetworksPrune(ctx, f)
	if err != nil {
		return network.PruneReport{}, fmt.Errorf("failed to prune networks: %w", err)
	}
	return report, nil
}
