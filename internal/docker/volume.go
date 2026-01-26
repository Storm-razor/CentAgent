package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

type ListVolumesOptions struct {
	// Filters 列表过滤条件，key/value 语义与 Docker Engine API 一致。
	Filters map[string][]string
}

// VolumeSummary 数据卷列表的简化信息（用于 list 输出）。
type VolumeSummary struct {
	// Name 卷名。
	Name string `json:"name"`
	// Driver 卷驱动（常见为 local）。
	Driver string `json:"driver"`
	// Mountpoint 卷在宿主机上的挂载路径（由 Docker 管理）。
	Mountpoint string `json:"mountpoint"`
	// Labels 卷标签。
	Labels map[string]string `json:"labels"`
	// Scope 卷作用域（如 local）。
	Scope string `json:"scope"`
	// CreatedAt 创建时间（字符串）。
	CreatedAt string `json:"created_at"`
}

func ListVolumes(ctx context.Context, opts ListVolumesOptions) ([]VolumeSummary, error) {
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	listOpts := volume.ListOptions{
		Filters: filters.NewArgs(),
	}
	for k, vs := range opts.Filters {
		for _, v := range vs {
			listOpts.Filters.Add(k, v)
		}
	}

	resp, err := cli.VolumeList(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	result := make([]VolumeSummary, 0, len(resp.Volumes))
	for _, v := range resp.Volumes {
		if v == nil {
			continue
		}
		result = append(result, VolumeSummary{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			Labels:     v.Labels,
			Scope:      v.Scope,
			CreatedAt:  v.CreatedAt,
		})
	}
	return result, nil
}

type CreateVolumeOptions struct {
	// Name 卷名。
	Name string
	// Driver 卷驱动（默认 local）。
	Driver string
	// Labels 卷标签。
	Labels map[string]string
	// DriverOpts 驱动相关的额外选项。
	DriverOpts map[string]string
}

func CreateVolume(ctx context.Context, opts CreateVolumeOptions) (volume.Volume, error) {
	cli, err := GetClient()
	if err != nil {
		return volume.Volume{}, err
	}

	created, err := cli.VolumeCreate(ctx, volume.CreateOptions{
		Name:       opts.Name,
		Driver:     opts.Driver,
		DriverOpts: opts.DriverOpts,
		Labels:     opts.Labels,
	})
	if err != nil {
		return volume.Volume{}, fmt.Errorf("failed to create volume %s: %w", opts.Name, err)
	}
	return created, nil
}

func InspectVolume(ctx context.Context, name string) (volume.Volume, error) {
	cli, err := GetClient()
	if err != nil {
		return volume.Volume{}, err
	}
	inspected, err := cli.VolumeInspect(ctx, name)
	if err != nil {
		return volume.Volume{}, fmt.Errorf("failed to inspect volume %s: %w", name, err)
	}
	return inspected, nil
}

type RemoveVolumeOptions struct {
	// Force 是否强制删除（即使正在被使用也尝试删除）。
	Force bool
}

func RemoveVolume(ctx context.Context, name string, opts RemoveVolumeOptions) error {
	cli, err := GetClient()
	if err != nil {
		return err
	}
	if err := cli.VolumeRemove(ctx, name, opts.Force); err != nil {
		return fmt.Errorf("failed to remove volume %s: %w", name, err)
	}
	return nil
}

func PruneVolumes(ctx context.Context, filterMap map[string][]string) (volume.PruneReport, error) {
	cli, err := GetClient()
	if err != nil {
		return volume.PruneReport{}, err
	}
	f := filters.NewArgs()
	for k, vs := range filterMap {
		for _, v := range vs {
			f.Add(k, v)
		}
	}
	report, err := cli.VolumesPrune(ctx, f)
	if err != nil {
		return volume.PruneReport{}, fmt.Errorf("failed to prune volumes: %w", err)
	}
	return report, nil
}
