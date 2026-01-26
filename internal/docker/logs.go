package docker

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// GetContainerLogsOptions 定义获取日志的参数
type GetContainerLogsOptions struct {
	ContainerID string `json:"container_id"`
	Tail        string `json:"tail"`
	Since       string `json:"since"`
	Details     bool   `json:"details"`
}

// GetContainerLogs 获取容器日志 (stdout + stderr)
func GetContainerLogs(ctx context.Context, opts GetContainerLogsOptions) (string, error) {
	tty := false
	if info, err := InspectContainer(ctx, opts.ContainerID); err == nil && info != nil && info.Config != nil {
		tty = info.Config.Tty
	}

	cli, err := GetClient()
	if err != nil {
		return "", err
	}

	logOpts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       opts.Tail,
		Since:      opts.Since,
		Details:    opts.Details,
	}
	if logOpts.Tail == "" {
		logOpts.Tail = "50"
	}

	reader, err := cli.ContainerLogs(ctx, opts.ContainerID, logOpts)
	if err != nil {
		return "", fmt.Errorf("failed to get logs for %s: %w", opts.ContainerID, err)
	}
	defer reader.Close()

	var result string
	if tty {
		body, err := io.ReadAll(reader)
		if err != nil {
			return "", fmt.Errorf("failed to read logs for %s: %w", opts.ContainerID, err)
		}
		result = fmt.Sprintf("=== LOGS ===\n%s", string(body))
	} else {
		var outBuf, errBuf strings.Builder

		// 仅在非 TTY 容器上使用 stdcopy 解析多路复用流
		if _, err := stdcopy.StdCopy(&outBuf, &errBuf, reader); err != nil {
			return "", fmt.Errorf("stdcopy failed for %s: %w", opts.ContainerID, err)
		}
		result = fmt.Sprintf("=== STDOUT ===\n%s\n=== STDERR ===\n%s", outBuf.String(), errBuf.String())
	}

	// 简单的截断保护
	if len(result) > 10000 {
		result = result[len(result)-10000:]
		result = "...(truncated)...\n" + result
	}

	return result, nil
}

// ContainerLogs 获取容器日志流
func ContainerLogs(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error) {
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}
	return cli.ContainerLogs(ctx, containerID, opts)
}
