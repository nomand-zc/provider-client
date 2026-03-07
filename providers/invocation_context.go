package providers

import "context"

// Invocation 调用上下文
type Invocation struct {
	RequestID string `json:"requestID"`
	Usage     Usage  `json:"usage"`
}

type invocationKey struct{}

// EnsureInvocationContext 创建调用上下文
func EnsureInvocationContext(ctx context.Context) (context.Context, *Invocation) {
	if ctx == nil {
		ctx = context.Background()
	}
	inv := GetInvocation(ctx)
	if inv != nil {
		return ctx, inv
	}
	inv = &Invocation{}
	return context.WithValue(ctx, invocationKey{}, inv), inv
}

// GetInvocation 获取调用上下文
func GetInvocation(ctx context.Context) *Invocation {
	if ctx == nil {
		return nil
	}
	if inv, ok := ctx.Value(invocationKey{}).(*Invocation); ok {
		return inv
	}
	return nil
}
