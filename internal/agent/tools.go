package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/wwwzy/CentAgent/internal/docker"
)

// ListContainersTool 列出容器
type ListContainersTool struct{}

func (t *ListContainersTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "list_containers",
		Desc: "List Docker containers. You can filter by status or limit the number of results.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"all" :{
				Desc: "Show all containers (default shows just running)",
				Type: schema.Boolean,
				Required: false,
			},
			"limit":{
				Desc: "Limit the number of containers shown",
				Type: schema.Integer,
				Required: false,
			},
			"status":{
				Desc: "Filter by status (e.g., 'running', 'exited')",
				Type: schema.String,
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
			"container_id":{
				Desc: "The ID or name of the container",
				Type: schema.String,
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
			"container_id":{
				Desc: "The ID or name of the container",
				Type: schema.String,
				Required: true,
			},
			"tail":{
				Desc: "Number of lines to show from the end of the logs (default '50')",
				Type: schema.String,
				Required: false,
			},
			"since":{
				Desc: "Show logs since timestamp (e.g. 2013-01-02T13:23:37Z) or relative (e.g. 42m for 42 minutes)",
				Type: schema.String,
				Required: false,
			},
			"details":{
				Desc: "Show extra details provided to logs",
				Type: schema.Boolean,
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

// GetTools 返回所有可用的工具列表
func GetTools() []tool.BaseTool {
	return []tool.BaseTool{
		&ListContainersTool{},
		&InspectContainerTool{},
		&GetContainerLogsTool{},
		&StartContainerTool{},
		&StopContainerTool{},
		&RestartContainerTool{},
	}
}

func GetToolsInfo(ctx context.Context) ([]*schema.ToolInfo,error) {
	tools := GetTools()
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
