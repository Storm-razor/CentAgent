package reactAgent

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/wwwzy/CentAgent/internal/storage"
)

const (
	auditTruncateLimit = 2048
)

// AuditedTool 是一个工具包装器，用于在工具执行前后记录审计日志
type AuditedTool struct {
	impl  tool.InvokableTool
	store *storage.Storage
}

// wrapWithAudit 将普通工具包装为带审计功能的工具
func wrapWithAudit(t tool.BaseTool, store *storage.Storage) tool.BaseTool {
	if store == nil {
		return t
	}
	// 确保工具实现了 InvokableRun 方法
	if it, ok := t.(tool.InvokableTool); ok {
		return &AuditedTool{impl: it, store: store}
	}
	// 如果未实现 InvokableRun，则不包装（虽然按约定都应该实现）
	return t
}

func (t *AuditedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.impl.Info(ctx)
}

func (t *AuditedTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 1. 获取工具信息（主要是 Action 名）
	info, err := t.impl.Info(ctx)
	action := "unknown"
	if err == nil && info != nil {
		action = info.Name
	}

	// 2. 准备审计记录
	traceID := GetTraceID(ctx)
	now := time.Now().UTC()
	record := &storage.AuditRecord{
		TraceID:    traceID,
		Action:     action,
		ParamsJSON: truncate(argumentsInJSON, auditTruncateLimit),
		Status:     "running",
		StartedAt:  now,
	}

	// 3. 插入初始记录（Status=running）
	// 注意：如果插入失败，我们通常选择打印日志但不阻断工具执行
	if err := t.store.InsertAuditRecord(ctx, record); err != nil {
		fmt.Printf("[WARN] Failed to insert audit record: %v\n", err)
	}

	// 4. 执行原始工具逻辑
	// 增加容错：如果参数 JSON 不完整（例如只包含 { ），尝试补全为 {}
	safeArgs := argumentsInJSON
	if safeArgs == "{" || safeArgs == "" {
		safeArgs = "{}"
	}
	result, runErr := t.impl.InvokableRun(ctx, safeArgs, opts...)

	// 5. 更新审计记录
	finishedAt := time.Now().UTC()
	status := "success"
	var errMsg *string
	var resultJSON *string

	if runErr != nil {
		status = "failed"
		e := truncate(runErr.Error(), auditTruncateLimit)
		errMsg = &e
	} else {
		r := truncate(result, auditTruncateLimit)
		resultJSON = &r
	}

	// 只有在 Insert 成功且有了 ID 后，才能 Update
	if record.ID != 0 {
		update := storage.AuditUpdate{
			Status:       &status,
			ResultJSON:   resultJSON,
			ErrorMessage: errMsg,
			FinishedAt:   &finishedAt,
		}
		if err := t.store.UpdateAuditRecord(ctx, record.ID, update); err != nil {
			fmt.Printf("[WARN] Failed to update audit record: %v\n", err)
		}
	}

	return result, runErr
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "...(truncated)"
}
