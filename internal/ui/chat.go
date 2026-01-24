package ui

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/wwwzy/CentAgent/internal/agent"
)

type ChatBackend interface {
	Invoke(ctx context.Context, state agent.AgentState, opts ...compose.Option) (agent.AgentState, error)
}

type ChatUI interface {
	Run(ctx context.Context, backend ChatBackend, initial agent.AgentState, opts ChatOptions) error
}

type ChatOptions struct {
	ConfirmTools bool
}

func DefaultInitialState() agent.AgentState {
	return agent.AgentState{
		Messages: nil,
		Context:  map[string]interface{}{},
	}
}
