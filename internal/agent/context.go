package agent

import (
	"context"
)

type traceIDKey struct{}

// WithTraceID 将 TraceID 注入 context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// GetTraceID 从 context 获取 TraceID
func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey{}).(string); ok {
		return v
	}
	return ""
}
