package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/wwwzy/CentAgent/internal/docker"
	"github.com/wwwzy/CentAgent/internal/storage"
)

const (
	maxStatsRowsPerTool = 200
	maxLogsRowsPerTool  = 200
)

// ListContainersTool 列出容器
type ListContainersTool struct{}

func (t *ListContainersTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "list_containers",
		Desc: "List Docker containers. You can filter by status or limit the number of results.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"all": {
				Desc:     "Show all containers (default shows just running)",
				Type:     schema.Boolean,
				Required: false,
			},
			"limit": {
				Desc:     "Limit the number of containers shown",
				Type:     schema.Integer,
				Required: false,
			},
			"status": {
				Desc:     "Filter by status (e.g., 'running', 'exited')",
				Type:     schema.String,
				Required: false,
			},
		}),
	}, nil
}

func (t *ListContainersTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args docker.ListContainersOptions
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	// 调试：打印解析后的参数
	fmt.Printf("[DEBUG] ListContainers args: %+v\n", args)

	containers, err := docker.ListContainers(ctx, args)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(containers)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

// InspectContainerTool 查看容器详情
type InspectContainerTool struct{}

func (t *InspectContainerTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "inspect_container",
		Desc: "Get detailed information about a container.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"container_id": {
				Desc:     "The ID or name of the container",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *InspectContainerTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	// 调试：打印解析后的参数
	fmt.Printf("[DEBUG] InspectContainer args: %+v\n", args)

	info, err := docker.InspectContainer(ctx, args.ContainerID)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

// GetContainerLogsTool 获取容器日志
type GetContainerLogsTool struct{}

func (t *GetContainerLogsTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_container_logs",
		Desc: "Get logs from a container.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"container_id": {
				Desc:     "The ID or name of the container",
				Type:     schema.String,
				Required: true,
			},
			"tail": {
				Desc:     "Number of lines to show from the end of the logs (default '50')",
				Type:     schema.String,
				Required: false,
			},
			"since": {
				Desc:     "Show logs since timestamp (e.g. 2013-01-02T13:23:37Z) or relative (e.g. 42m for 42 minutes)",
				Type:     schema.String,
				Required: false,
			},
			"details": {
				Desc:     "Show extra details provided to logs",
				Type:     schema.Boolean,
				Required: false,
			},
		}),
	}, nil
}

func (t *GetContainerLogsTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args docker.GetContainerLogsOptions
	// 注意：JSON 中的字段名需要匹配 struct tag，如果 struct 没有 json tag，则默认匹配字段名
	// 这里假设 LLM 会生成 snake_case 的参数，我们需要确保能正确映射
	// 为了保险起见，我们可以定义一个临时的结构体来接收 JSON

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	// 调试：打印解析后的参数
	fmt.Printf("[DEBUG] GetContainerLogs args: %+v\n", args)

	logs, err := docker.GetContainerLogs(ctx, args)
	if err != nil {
		return "", err
	}
	return logs, nil
}

// StartContainerTool 启动容器
type StartContainerTool struct{}

func (t *StartContainerTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "start_container",
		Desc: "Start a container.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"container_id": {
				Desc:     "The ID or name of the container",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *StartContainerTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if err := docker.StartContainer(ctx, args.ContainerID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Container %s started successfully", args.ContainerID), nil
}

// StopContainerTool 停止容器
type StopContainerTool struct{}

func (t *StopContainerTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "stop_container",
		Desc: "Stop a container.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"container_id": {
				Desc:     "The ID or name of the container",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *StopContainerTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if err := docker.StopContainer(ctx, args.ContainerID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Container %s stopped successfully", args.ContainerID), nil
}

// RestartContainerTool 重启容器
type RestartContainerTool struct{}

func (t *RestartContainerTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "restart_container",
		Desc: "Restart a container.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"container_id": {
				Desc:     "The ID or name of the container",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *RestartContainerTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if err := docker.RestartContainer(ctx, args.ContainerID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Container %s restarted successfully", args.ContainerID), nil
}

// RunContainerTool 从镜像创建并启动容器
type RunContainerTool struct{}

func (t *RunContainerTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "run_container",
		Desc: "Create and start a container from an image.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"image": {
				Desc:     "Image reference (e.g. nginx:alpine)",
				Type:     schema.String,
				Required: true,
			},
			"name": {
				Desc:     "Optional container name",
				Type:     schema.String,
				Required: false,
			},
			"cmd": {
				Desc:     "Optional command override (array of strings)",
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Required: false,
			},
			"env": {
				Desc:     "Optional environment variables (array of KEY=VALUE)",
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Required: false,
			},
			"working_dir": {
				Desc:     "Optional working directory",
				Type:     schema.String,
				Required: false,
			},
			"auto_remove": {
				Desc:     "Auto remove the container when it exits",
				Type:     schema.Boolean,
				Required: false,
			},
			"restart_policy": {
				Desc:     "Restart policy: no/always/unless-stopped/on-failure",
				Type:     schema.String,
				Required: false,
			},
			"binds": {
				Desc:     "Volume binds (array), syntax like docker -v (e.g. myvol:/data or /host:/data:ro)",
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Required: false,
			},
			"network": {
				Desc:     "Optional network name/ID to connect at create time",
				Type:     schema.String,
				Required: false,
			},
			"publish": {
				Desc:     "Port publish rules (array), like docker -p. Examples: 8080:80, 127.0.0.1:8080:80/tcp",
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Required: false,
			},
			"pull_if_missing": {
				Desc:     "Pull the image if it is not available locally",
				Type:     schema.Boolean,
				Required: false,
			},
		}),
	}, nil
}

func (t *RunContainerTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		Image         string   `json:"image"`
		Name          string   `json:"name"`
		Cmd           []string `json:"cmd"`
		Env           []string `json:"env"`
		WorkingDir    string   `json:"working_dir"`
		AutoRemove    bool     `json:"auto_remove"`
		RestartPolicy string   `json:"restart_policy"`
		Binds         []string `json:"binds"`
		Network       string   `json:"network"`
		Publish       []string `json:"publish"`
		PullIfMissing bool     `json:"pull_if_missing"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] RunContainer args: %+v\n", args)

	res, err := docker.RunContainerFromImage(ctx, docker.RunContainerFromImageOptions{
		Image:         args.Image,
		Name:          args.Name,
		Cmd:           args.Cmd,
		Env:           args.Env,
		WorkingDir:    args.WorkingDir,
		AutoRemove:    args.AutoRemove,
		RestartPolicy: args.RestartPolicy,
		Binds:         args.Binds,
		Network:       args.Network,
		Publish:       args.Publish,
		PullIfMissing: args.PullIfMissing,
	})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

type ListImagesTool struct{}

func (t *ListImagesTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "list_images",
		Desc: "List Docker images.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"all": {
				Desc:     "Show all images (default hides intermediate images)",
				Type:     schema.Boolean,
				Required: false,
			},
		}),
	}, nil
}

func (t *ListImagesTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		All bool `json:"all"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] ListImages args: %+v\n", args)

	images, err := docker.ListImages(ctx, docker.ListImagesOptions{All: args.All})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(images)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

type InspectImageTool struct{}

func (t *InspectImageTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "inspect_image",
		Desc: "Get detailed information about an image.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"ref": {
				Desc:     "The image reference (name:tag, digest, or ID)",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *InspectImageTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] InspectImage args: %+v\n", args)

	info, err := docker.InspectImage(ctx, args.Ref)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

type PullImageTool struct{}

func (t *PullImageTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "pull_image",
		Desc: "Pull an image from a registry.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"ref": {
				Desc:     "The image reference to pull (e.g. nginx:alpine)",
				Type:     schema.String,
				Required: true,
			},
			"platform": {
				Desc:     "Optional platform (e.g. linux/amd64)",
				Type:     schema.String,
				Required: false,
			},
		}),
	}, nil
}

func (t *PullImageTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		Ref      string `json:"ref"`
		Platform string `json:"platform"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] PullImage args: %+v\n", args)

	out, err := docker.PullImage(ctx, docker.PullImageOptions{Ref: args.Ref, Platform: args.Platform})
	if err != nil {
		return "", err
	}
	return out, nil
}

type RemoveImageTool struct{}

func (t *RemoveImageTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "remove_image",
		Desc: "Remove an image.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"ref": {
				Desc:     "The image reference (name:tag, digest, or ID)",
				Type:     schema.String,
				Required: true,
			},
			"force": {
				Desc:     "Force removal of the image",
				Type:     schema.Boolean,
				Required: false,
			},
			"prune_children": {
				Desc:     "Remove untagged parent images",
				Type:     schema.Boolean,
				Required: false,
			},
		}),
	}, nil
}

func (t *RemoveImageTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		Ref           string `json:"ref"`
		Force         bool   `json:"force"`
		PruneChildren bool   `json:"prune_children"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] RemoveImage args: %+v\n", args)

	deleted, err := docker.RemoveImage(ctx, args.Ref, docker.RemoveImageOptions{
		Force:         args.Force,
		PruneChildren: args.PruneChildren,
	})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(deleted)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

type ListNetworksTool struct{}

func (t *ListNetworksTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "list_networks",
		Desc:        "List Docker networks.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}

func (t *ListNetworksTool) InvokableRun(ctx context.Context, _ string, _ ...tool.Option) (string, error) {
	networks, err := docker.ListNetworks(ctx, docker.ListNetworksOptions{})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(networks)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

type CreateNetworkTool struct{}

func (t *CreateNetworkTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "create_network",
		Desc: "Create a Docker network.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name": {
				Desc:     "Network name",
				Type:     schema.String,
				Required: true,
			},
			"driver": {
				Desc:     "Network driver (e.g. bridge, overlay)",
				Type:     schema.String,
				Required: false,
			},
			"internal": {
				Desc:     "Restrict external access to the network",
				Type:     schema.Boolean,
				Required: false,
			},
			"attachable": {
				Desc:     "Enable manual container attachment",
				Type:     schema.Boolean,
				Required: false,
			},
		}),
	}, nil
}

func (t *CreateNetworkTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		Name       string `json:"name"`
		Driver     string `json:"driver"`
		Internal   bool   `json:"internal"`
		Attachable bool   `json:"attachable"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] CreateNetwork args: %+v\n", args)

	resp, err := docker.CreateNetwork(ctx, docker.CreateNetworkOptions{
		Name:       args.Name,
		Driver:     args.Driver,
		Internal:   args.Internal,
		Attachable: args.Attachable,
	})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

type InspectNetworkTool struct{}

func (t *InspectNetworkTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "inspect_network",
		Desc: "Get detailed information about a network.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"network_id": {
				Desc:     "The network ID or name",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *InspectNetworkTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		NetworkID string `json:"network_id"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] InspectNetwork args: %+v\n", args)

	info, err := docker.InspectNetwork(ctx, args.NetworkID)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

type ConnectNetworkTool struct{}

func (t *ConnectNetworkTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "connect_network",
		Desc: "Connect a container to a network.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"network_id": {
				Desc:     "The network ID or name",
				Type:     schema.String,
				Required: true,
			},
			"container_id": {
				Desc:     "The container ID or name",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *ConnectNetworkTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		NetworkID   string `json:"network_id"`
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] ConnectNetwork args: %+v\n", args)

	if err := docker.ConnectNetwork(ctx, args.NetworkID, docker.ConnectNetworkOptions{ContainerID: args.ContainerID}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Connected container %s to network %s", args.ContainerID, args.NetworkID), nil
}

type DisconnectNetworkTool struct{}

func (t *DisconnectNetworkTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "disconnect_network",
		Desc: "Disconnect a container from a network.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"network_id": {
				Desc:     "The network ID or name",
				Type:     schema.String,
				Required: true,
			},
			"container_id": {
				Desc:     "The container ID or name",
				Type:     schema.String,
				Required: true,
			},
			"force": {
				Desc:     "Force disconnect",
				Type:     schema.Boolean,
				Required: false,
			},
		}),
	}, nil
}

func (t *DisconnectNetworkTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		NetworkID   string `json:"network_id"`
		ContainerID string `json:"container_id"`
		Force       bool   `json:"force"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] DisconnectNetwork args: %+v\n", args)

	if err := docker.DisconnectNetwork(ctx, args.NetworkID, docker.DisconnectNetworkOptions{ContainerID: args.ContainerID, Force: args.Force}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Disconnected container %s from network %s", args.ContainerID, args.NetworkID), nil
}

type RemoveNetworkTool struct{}

func (t *RemoveNetworkTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "remove_network",
		Desc: "Remove a Docker network.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"network_id": {
				Desc:     "The network ID or name",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *RemoveNetworkTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		NetworkID string `json:"network_id"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] RemoveNetwork args: %+v\n", args)

	if err := docker.RemoveNetwork(ctx, args.NetworkID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Network %s removed successfully", args.NetworkID), nil
}

type ListVolumesTool struct{}

func (t *ListVolumesTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "list_volumes",
		Desc:        "List Docker volumes.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}

func (t *ListVolumesTool) InvokableRun(ctx context.Context, _ string, _ ...tool.Option) (string, error) {
	volumes, err := docker.ListVolumes(ctx, docker.ListVolumesOptions{})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(volumes)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

type CreateVolumeTool struct{}

func (t *CreateVolumeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "create_volume",
		Desc: "Create a Docker volume.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name": {
				Desc:     "Volume name",
				Type:     schema.String,
				Required: true,
			},
			"driver": {
				Desc:     "Volume driver (default is local)",
				Type:     schema.String,
				Required: false,
			},
		}),
	}, nil
}

func (t *CreateVolumeTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		Name   string `json:"name"`
		Driver string `json:"driver"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] CreateVolume args: %+v\n", args)

	created, err := docker.CreateVolume(ctx, docker.CreateVolumeOptions{Name: args.Name, Driver: args.Driver})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(created)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

type InspectVolumeTool struct{}

func (t *InspectVolumeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "inspect_volume",
		Desc: "Get detailed information about a volume.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name": {
				Desc:     "Volume name",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *InspectVolumeTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] InspectVolume args: %+v\n", args)

	info, err := docker.InspectVolume(ctx, args.Name)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

type RemoveVolumeTool struct{}

func (t *RemoveVolumeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "remove_volume",
		Desc: "Remove a Docker volume.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name": {
				Desc:     "Volume name",
				Type:     schema.String,
				Required: true,
			},
			"force": {
				Desc:     "Force removal of the volume",
				Type:     schema.Boolean,
				Required: false,
			},
		}),
	}, nil
}

func (t *RemoveVolumeTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		Name  string `json:"name"`
		Force bool   `json:"force"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	fmt.Printf("[DEBUG] RemoveVolume args: %+v\n", args)

	if err := docker.RemoveVolume(ctx, args.Name, docker.RemoveVolumeOptions{Force: args.Force}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Volume %s removed successfully", args.Name), nil
}

type QueryContainerStatsTool struct {
	store *storage.Storage
}

func (t *QueryContainerStatsTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "query_container_stats",
		Desc: "Query a small slice of historical container stats from the CentAgent database. This tool is designed to be called multiple times with different time windows or limits to avoid fetching too much data at once.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"container_id": {
				Desc:     "Optional container ID to filter (exact match)",
				Type:     schema.String,
				Required: false,
			},
			"container_name": {
				Desc:     "Optional container name to filter (exact match)",
				Type:     schema.String,
				Required: false,
			},
			"from": {
				Desc:     "Optional start time (RFC3339) or duration like 10m/1h (means now-10m/now-1h)",
				Type:     schema.String,
				Required: false,
			},
			"to": {
				Desc:     "Optional end time (RFC3339) or duration like 10m/1h (means now-10m/now-1h)",
				Type:     schema.String,
				Required: false,
			},
			"limit": {
				Desc:     "Limit the number of rows returned (default 200, max 200). Use multiple calls with different time ranges for more data.",
				Type:     schema.Integer,
				Required: false,
			},
			"desc": {
				Desc:     "Sort by collected_at descending (latest first)",
				Type:     schema.Boolean,
				Required: false,
			},
		}),
	}, nil
}

func (t *QueryContainerStatsTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	if t == nil || t.store == nil {
		return "", fmt.Errorf("storage not initialized")
	}
	var args struct {
		ContainerID   string `json:"container_id"`
		ContainerName string `json:"container_name"`
		From          string `json:"from"`
		To            string `json:"to"`
		Limit         int    `json:"limit"`
		Desc          bool   `json:"desc"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	normalizedContainerID := strings.TrimSpace(args.ContainerID)
	normalizedContainerName := strings.TrimSpace(args.ContainerName)
	limit := args.Limit
	if limit <= 0 || limit > maxStatsRowsPerTool {
		limit = maxStatsRowsPerTool
	}
	q := storage.StatsQuery{
		ContainerID:   normalizedContainerID,
		ContainerName: normalizedContainerName,
		Limit:         limit,
		Desc:          args.Desc,
	}
	if s := strings.TrimSpace(args.From); s != "" {
		tm, err := parseTimeArg(s, time.Now().UTC())
		if err != nil {
			return "", err
		}
		q.From = &tm
	}
	if s := strings.TrimSpace(args.To); s != "" {
		tm, err := parseTimeArg(s, time.Now().UTC())
		if err != nil {
			return "", err
		}
		q.To = &tm
	}

	stats, err := t.queryStatsWithFallback(ctx, q)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(stats)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

func (t *QueryContainerStatsTool) queryStatsWithFallback(ctx context.Context, q storage.StatsQuery) ([]storage.ContainerStat, error) {
	candidatesID := containerIDCandidates(q.ContainerID)
	candidatesName := containerNameCandidates(q.ContainerName)

	if len(candidatesID) == 0 && len(candidatesName) == 0 {
		return t.store.QueryContainerStats(ctx, q)
	}

	if len(candidatesID) == 0 {
		candidatesID = []string{""}
	}
	if len(candidatesName) == 0 {
		candidatesName = []string{""}
	}

	var lastErr error
	for _, id := range candidatesID {
		for _, name := range candidatesName {
			try := q
			try.ContainerID = id
			try.ContainerName = name
			out, err := t.store.QueryContainerStats(ctx, try)
			if err != nil {
				lastErr = err
				continue
			}
			if len(out) > 0 {
				return out, nil
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return []storage.ContainerStat{}, nil
}

type QueryContainerLogsTool struct {
	store *storage.Storage
}

func (t *QueryContainerLogsTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "query_container_logs",
		Desc: "Query a small slice of historical container logs from the CentAgent database. This tool is designed to be called multiple times with different time windows or limits to avoid fetching too much data at once.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"container_id": {
				Desc:     "Optional container ID to filter (exact match)",
				Type:     schema.String,
				Required: false,
			},
			"container_name": {
				Desc:     "Optional container name to filter (exact match)",
				Type:     schema.String,
				Required: false,
			},
			"from": {
				Desc:     "Optional start time (RFC3339) or duration like 10m/1h (means now-10m/now-1h)",
				Type:     schema.String,
				Required: false,
			},
			"to": {
				Desc:     "Optional end time (RFC3339) or duration like 10m/1h (means now-10m/now-1h)",
				Type:     schema.String,
				Required: false,
			},
			"level": {
				Desc:     "Optional log level to filter (exact match, e.g. ERROR/WARN/INFO)",
				Type:     schema.String,
				Required: false,
			},
			"source": {
				Desc:     "Optional log source to filter (exact match, e.g. stdout/stderr)",
				Type:     schema.String,
				Required: false,
			},
			"contains": {
				Desc:     "Optional substring to search within message (SQL LIKE)",
				Type:     schema.String,
				Required: false,
			},
			"limit": {
				Desc:     "Limit the number of rows returned (default 200, max 200). Use multiple calls with different time ranges for more data.",
				Type:     schema.Integer,
				Required: false,
			},
			"desc": {
				Desc:     "Sort by timestamp descending (latest first)",
				Type:     schema.Boolean,
				Required: false,
			},
		}),
	}, nil
}

func (t *QueryContainerLogsTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	if t == nil || t.store == nil {
		return "", fmt.Errorf("storage not initialized")
	}
	var args struct {
		ContainerID   string `json:"container_id"`
		ContainerName string `json:"container_name"`
		From          string `json:"from"`
		To            string `json:"to"`
		Level         string `json:"level"`
		Source        string `json:"source"`
		Contains      string `json:"contains"`
		Limit         int    `json:"limit"`
		Desc          bool   `json:"desc"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	normalizedContainerID := strings.TrimSpace(args.ContainerID)
	normalizedContainerName := strings.TrimSpace(args.ContainerName)

	limit := args.Limit
	if limit <= 0 || limit > maxLogsRowsPerTool {
		limit = maxLogsRowsPerTool
	}

	q := storage.LogQuery{
		ContainerID:   normalizedContainerID,
		ContainerName: normalizedContainerName,
		Level:         strings.TrimSpace(args.Level),
		Source:        strings.TrimSpace(args.Source),
		Contains:      strings.TrimSpace(args.Contains),
		Limit:         limit,
		Desc:          args.Desc,
	}
	if s := strings.TrimSpace(args.From); s != "" {
		tm, err := parseTimeArg(s, time.Now().UTC())
		if err != nil {
			return "", err
		}
		q.From = &tm
	}
	if s := strings.TrimSpace(args.To); s != "" {
		tm, err := parseTimeArg(s, time.Now().UTC())
		if err != nil {
			return "", err
		}
		q.To = &tm
	}

	logs, err := t.queryLogsWithFallback(ctx, q)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(logs)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

func (t *QueryContainerLogsTool) queryLogsWithFallback(ctx context.Context, q storage.LogQuery) ([]storage.ContainerLog, error) {
	candidatesID := containerIDCandidates(q.ContainerID)
	candidatesName := containerNameCandidates(q.ContainerName)

	if len(candidatesID) == 0 && len(candidatesName) == 0 {
		return t.store.QueryContainerLogs(ctx, q)
	}

	if len(candidatesID) == 0 {
		candidatesID = []string{""}
	}
	if len(candidatesName) == 0 {
		candidatesName = []string{""}
	}

	var lastErr error
	for _, id := range candidatesID {
		for _, name := range candidatesName {
			try := q
			try.ContainerID = id
			try.ContainerName = name
			out, err := t.store.QueryContainerLogs(ctx, try)
			if err != nil {
				lastErr = err
				continue
			}
			if len(out) > 0 {
				return out, nil
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return []storage.ContainerLog{}, nil
}

func parseTimeArg(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("time string is empty")
	}
	if d, err := time.ParseDuration(s); err == nil {
		if d > 0 {
			d = -d
		}
		return now.Add(d).UTC(), nil
	}
	if tm, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return tm.UTC(), nil
	}
	if tm, err := time.Parse(time.RFC3339, s); err == nil {
		return tm.UTC(), nil
	}
	if sec, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(sec, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid time format: %s (use RFC3339 or duration like 10m)", s)
}

func containerNameCandidates(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	base := strings.TrimPrefix(name, "/")
	out := make([]string, 0, 2)
	seen := map[string]struct{}{}

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	add(name)
	add(base)
	add("/" + base)
	return out
}

func containerIDCandidates(id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	out := make([]string, 0, 2)
	seen := map[string]struct{}{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(id)
	if len(id) > 12 {
		add(id[:12])
	}
	return out
}

// GetTools 返回所有可用的工具列表
func GetTools(store *storage.Storage) []tool.BaseTool {
	tools := []tool.BaseTool{
		&ListContainersTool{},
		&InspectContainerTool{},
		&GetContainerLogsTool{},
		&RunContainerTool{},
		&StartContainerTool{},
		&StopContainerTool{},
		&RestartContainerTool{},
		&ListImagesTool{},
		&InspectImageTool{},
		&PullImageTool{},
		&RemoveImageTool{},
		&ListNetworksTool{},
		&CreateNetworkTool{},
		&InspectNetworkTool{},
		&ConnectNetworkTool{},
		&DisconnectNetworkTool{},
		&RemoveNetworkTool{},
		&ListVolumesTool{},
		&CreateVolumeTool{},
		&InspectVolumeTool{},
		&RemoveVolumeTool{},
	}
	if store != nil {
		tools = append(tools, &QueryContainerStatsTool{store: store}, &QueryContainerLogsTool{store: store})
	}
	return tools
}

func GetToolsInfo(ctx context.Context, store *storage.Storage) ([]*schema.ToolInfo, error) {
	tools := GetTools(store)
	toolInfos := make([]*schema.ToolInfo, 0, len(tools))
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get tool info: %w", err)
		}
		toolInfos = append(toolInfos, info)
	}
	return toolInfos, nil
}
