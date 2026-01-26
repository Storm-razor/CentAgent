package docker

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	v1 "github.com/moby/docker-image-spec/specs-go/v1"
)

type ListImagesOptions struct {
	// All 是否包含中间层镜像（默认 false）。
	All bool
	// Filters 列表过滤条件，key/value 语义与 Docker Engine API 一致。
	Filters map[string][]string
}

// ImageSummary 镜像列表的简化信息（用于 list 输出）。
type ImageSummary struct {
	// ID 镜像 ID（列表场景会做截断便于阅读）。
	ID string `json:"id"`
	// RepoTags 本地镜像标签（如 nginx:alpine）。
	RepoTags []string `json:"repo_tags"`
	// RepoDigests 本地已知的 manifest digest（可能为空）。
	RepoDigests []string `json:"repo_digests"`
	// Created 创建时间（Unix 秒）。
	Created int64 `json:"created"`
	// Size 镜像大小（字节）。
	Size int64 `json:"size"`
}

func ListImages(ctx context.Context, opts ListImagesOptions) ([]ImageSummary, error) {
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	listOpts := image.ListOptions{
		All:     opts.All,
		Filters: filters.NewArgs(),
	}
	for k, vs := range opts.Filters {
		for _, v := range vs {
			listOpts.Filters.Add(k, v)
		}
	}

	images, err := cli.ImageList(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	result := make([]ImageSummary, 0, len(images))
	for _, img := range images {
		result = append(result, ImageSummary{
			ID:          truncateID(img.ID),
			RepoTags:    img.RepoTags,
			RepoDigests: img.RepoDigests,
			Created:     img.Created,
			Size:        img.Size,
		})
	}
	return result, nil
}

// InspectImageDetail 镜像 inspect 的简化信息（用于给 Agent/CLI 输出）。
type InspectImageDetail struct {
	// ID 镜像 content-addressable ID。
	ID string `json:"id"`
	// RepoTags 镜像标签列表。
	RepoTags []string `json:"repo_tags"`
	// RepoDigests 镜像 digest 列表。
	RepoDigests []string `json:"repo_digests"`
	// Created 创建时间（RFC3339 字符串）。
	Created string `json:"created"`
	// Size 镜像大小（字节）。
	Size int64 `json:"size"`
	// OS 镜像目标操作系统（如 linux）。
	OS string `json:"os"`
	// Architecture 镜像目标架构（如 amd64）。
	Architecture string `json:"architecture"`
	// Config 镜像配置（ENV/CMD/Entrypoint 等）。
	Config *v1.DockerOCIImageConfig `json:"config"`
	// RootFS RootFS 分层信息。
	RootFS types.RootFS `json:"root_fs"`
	// GraphDriver 本地存储驱动信息。
	GraphDriver types.GraphDriverData `json:"graph_driver"`
}

func InspectImage(ctx context.Context, ref string) (*InspectImageDetail, error) {
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	inspect, _, err := cli.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image %s: %w", ref, err)
	}

	return &InspectImageDetail{
		ID:           inspect.ID,
		RepoTags:     inspect.RepoTags,
		RepoDigests:  inspect.RepoDigests,
		Created:      inspect.Created,
		Size:         inspect.Size,
		OS:           inspect.Os,
		Architecture: inspect.Architecture,
		Config:       inspect.Config,
		RootFS:       inspect.RootFS,
		GraphDriver:  inspect.GraphDriver,
	}, nil
}

type PullImageOptions struct {
	// Ref 镜像引用（name:tag、digest、或镜像 ID）。
	Ref string
	// Platform 可选平台（如 linux/amd64）。
	Platform string
}

func PullImage(ctx context.Context, opts PullImageOptions) (string, error) {
	cli, err := GetClient()
	if err != nil {
		return "", err
	}

	ref := strings.TrimSpace(opts.Ref)
	if ref == "" {
		return "", fmt.Errorf("image ref is required")
	}

	pullOpts := image.PullOptions{}
	if strings.TrimSpace(opts.Platform) != "" {
		pullOpts.Platform = strings.TrimSpace(opts.Platform)
	}

	reader, err := cli.ImagePull(ctx, ref, pullOpts)
	if err != nil {
		return "", fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	defer reader.Close()

	var b strings.Builder
	if _, err := io.Copy(&b, reader); err != nil {
		return "", fmt.Errorf("failed to read image pull output: %w", err)
	}

	return truncateTail(b.String(), 2000), nil
}

type RemoveImageOptions struct {
	// Force 是否强制删除。
	Force bool
	// PruneChildren 是否同时删除无标签的父镜像。
	PruneChildren bool
}

func RemoveImage(ctx context.Context, ref string, opts RemoveImageOptions) ([]image.DeleteResponse, error) {
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	deleted, err := cli.ImageRemove(ctx, ref, image.RemoveOptions{
		Force:         opts.Force,
		PruneChildren: opts.PruneChildren,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to remove image %s: %w", ref, err)
	}
	return deleted, nil
}

func PruneImages(ctx context.Context, filterMap map[string][]string) (image.PruneReport, error) {
	cli, err := GetClient()
	if err != nil {
		return image.PruneReport{}, err
	}
	f := filters.NewArgs()
	for k, vs := range filterMap {
		for _, v := range vs {
			f.Add(k, v)
		}
	}
	report, err := cli.ImagesPrune(ctx, f)
	if err != nil {
		return image.PruneReport{}, fmt.Errorf("failed to prune images: %w", err)
	}
	return report, nil
}
