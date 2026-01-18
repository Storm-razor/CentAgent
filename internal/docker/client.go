package docker

import (
	"fmt"
	"sync"

	"github.com/docker/docker/client"
)

var (
	dockerCli *client.Client
	once      sync.Once
)

// GetClient 获取 Docker Client 单例
// 懒加载模式，第一次调用时初始化
func GetClient() (*client.Client, error) {
	var err error
	once.Do(func() {
		// 使用 FromEnv 自动读取环境变量 (DOCKER_HOST, etc.)
		// 并在 API 版本协商上自动适配
		dockerCli, err = client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return dockerCli, nil
}

// CloseClient 关闭 Docker Client 连接
// 建议在程序退出时调用
func CloseClient() error {
	if dockerCli != nil {
		return dockerCli.Close()
	}
	return nil
}
