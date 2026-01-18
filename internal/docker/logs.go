package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
)

// GetContainerLogsOptions 定义获取日志的参数
type GetContainerLogsOptions struct {
	ContainerID string
	Tail        string // "all" or number
	Since       string // timestamp or duration string
	Details    	bool
}

// GetContainerLogs 获取容器日志 (stdout + stderr)
func GetContainerLogs(ctx context.Context, opts GetContainerLogsOptions) (string, error) {
	cli, err := GetClient()
	if err != nil {
		return "", err
	}

	logOpts := types.ContainerLogsOptions{
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

	var outBuf, errBuf strings.Builder

	// 尝试解析多路复用流
	_, err = stdcopy.StdCopy(&outBuf, &errBuf, reader)
	if err != nil {
		// 如果失败（例如 tty=true），则直接读取
		return "", fmt.Errorf("stdcopy failed (container might be using TTY): %w", err)
	}

	result := fmt.Sprintf("=== STDOUT ===\n%s\n=== STDERR ===\n%s", outBuf.String(), errBuf.String())

	// 简单的截断保护
	if len(result) > 10000 {
		result = result[len(result)-10000:]
		result = "...(truncated)...\n" + result
	}

	return result, nil
}
