package client

import (
	"context"

	"github.com/nomand-zc/token101/provider-client/types"
)

// Converter 负责请求/响应的数据格式转换（序列化/反序列化）
type Converter interface {
	// ConvertRequest 将标准请求序列化为目标 API 格式的 JSON 字节切片
	ConvertRequest(ctx context.Context, creds []byte, req types.Request) ([]byte, error)
	// ConvertResponse 将目标 API 响应的 JSON 字节切片反序列化为标准响应格式
	ConvertResponse(ctx context.Context, body []byte) (*types.Response, error)
	// ConvertStreamChunk 将单行 SSE 流式数据（含 "data: " 前缀）解析为标准流式响应格式
	// 返回 nil 表示该行应被跳过（空行或 [DONE] 标记）
	ConvertStreamChunk(ctx context.Context, line []byte) (*types.Response, error)
}
