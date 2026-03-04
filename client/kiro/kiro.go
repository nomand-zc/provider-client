package kiro

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/gg/gcond"
	providerclient "github.com/nomand-zc/token101/provider-client"
	"github.com/nomand-zc/token101/provider-client/client"
	"github.com/nomand-zc/token101/provider-client/types"
)

// kiro implements the Model interface for Kiro CodeWhisperer API
type kiro struct {
	options    Options
	httpClient client.HTTPClient
	converter  client.Converter
}

// New creates a new Kiro client instance
func New(opts ...Option) providerclient.Model {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}

	return &kiro{
		options:    options,
		httpClient: client.DefaultNewHTTPClient(),
		converter:  NewKiroConverter(options),
	}
}

// buildURL 构建 Kiro API 请求 URL，优先使用凭证中的 region
func (k *kiro) buildURL(creds Credentials) string {
	region := gcond.If(creds.Region != "", creds.Region, k.options.defaultRegion)
	return fmt.Sprintf(k.options.url, region)
}

// buildHeaders 根据凭证和配置选项构建 Kiro API 所需的 HTTP 请求头
func (k *kiro) buildHeaders(creds Credentials) (map[string]string, error) {
	// 以 defaultOptions.headers 为基础构建 header map
	headers := make(map[string]string, len(defaultOptions.headers)+1)
	maps.Copy(headers, defaultOptions.headers)

	// 设置认证头
	headers["Authorization"] = fmt.Sprintf("Bearer %s", creds.AccessToken)

	// 设置每次请求唯一的调用 ID（UUID v4 格式）
	headers["amz-sdk-invocation-id"] = generateUUID()

	// 合并 options.headers 中的自定义 header（可覆盖默认值）
	maps.Copy(headers, k.options.headers)

	return headers, nil
}

// buildHTTPRequest 封装请求构建逻辑：序列化请求体 + 构建 URL + 创建 HTTP 请求 + 设置 header
func (k *kiro) buildHTTPRequest(ctx context.Context, account types.Account, req types.Request) (*http.Request, error) {
	// 将标准请求序列化为 Kiro 格式的 JSON 字节切片（凭证用于注入 profileArn）
	body, err := k.converter.ConvertRequest(ctx, account.Creds, req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	creds, err := ExtractCredentials(account.Creds)
	if err != nil {
		return nil, fmt.Errorf("failed to extract credentials: %w", err)
	}
	// 构建请求 URL
	url := k.buildURL(*creds)

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// 构建并设置请求头
	headers, err := k.buildHeaders(*creds)
	if err != nil {
		return nil, fmt.Errorf("failed to build headers: %w", err)
	}
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	return httpReq, nil
}

// GenerateContent implements the Model interface for synchronous content generation
func (k *kiro) GenerateContent(ctx context.Context, account types.Account, req types.Request) (*types.Response, error) {
	// 验证请求
	if err := validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 构建 HTTP 请求（含 body、URL 和 header）
	httpReq, err := k.buildHTTPRequest(ctx, account, req)
	if err != nil {
		return nil, err
	}

	// 发送 HTTP 请求
	resp, err := k.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Kiro API: %w", err)
	}

	// 读取并关闭响应体
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 检查响应状态码，非 2xx 时返回归一化的 HTTPError
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewHTTPError(resp.StatusCode, respBody)
	}

	// 将响应体字节切片转换为标准响应格式
	response, err := k.converter.ConvertResponse(ctx, respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to convert response: %w", err)
	}

	return response, nil
}

// GenerateContentStream implements the Model interface for streaming content generation
func (k *kiro) GenerateContentStream(ctx context.Context, account types.Account, req types.Request) (types.ResponseChain, error) {
	// 验证请求
	if err := validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 设置流式模式
	req.GenerationConfig.Stream = true

	// 构建 HTTP 请求（含 body、URL 和 header）
	httpReq, err := k.buildHTTPRequest(ctx, account, req)
	if err != nil {
		return nil, err
	}

	// 发送 HTTP 请求
	resp, err := k.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Kiro API: %w", err)
	}

	// 检查响应状态码，非 2xx 时读取 body 后返回归一化的 HTTPError
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, NewHTTPError(resp.StatusCode, errBody)
	}

	// 创建响应 channel
	chain, sender := types.NewResponseChain(10)

	// 在 goroutine 中处理流式响应
	go func() {
		defer close(sender)
		defer resp.Body.Close()

		// 检查是否为流式响应（使用 strings.HasPrefix 正确处理带参数的媒体类型）
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "text/event-stream") && !strings.HasPrefix(contentType, "application/x-ndjson") {
			response := k.handleNonStreamResponse(ctx, resp.Body)
			sender.Send(ctx, response)
			return
		}

		for response := range k.handleSSEStreamResponse(ctx, resp.Body) {
			if !sender.Send(ctx, response) {
				return
			}
			if response.Done || response.Error != nil {
				return
			}
		}
	}()

	return chain, nil
}

// handleNonStreamResponse 处理非流式响应：读取 body 并转换为标准响应格式后返回
func (k *kiro) handleNonStreamResponse(ctx context.Context, body io.Reader) *types.Response {
	respBody, err := io.ReadAll(body)
	if err != nil {
		return &types.Response{
			Error: &types.ResponseError{
				Code:    stringPtr("READ_ERROR"),
				Message: fmt.Sprintf("failed to read response body: %v", err),
			},
		}
	}
	response, err := k.converter.ConvertResponse(ctx, respBody)
	if err != nil {
		return &types.Response{
			Error: &types.ResponseError{
				Code:    stringPtr("PARSE_ERROR"),
				Message: fmt.Sprintf("failed to parse response: %v", err),
			},
		}
	}
	return response
}

// handleSSEStreamResponse 处理 SSE 流式响应：逐行读取并解析数据块，通过 ResponseChain 逐个返回
func (k *kiro) handleSSEStreamResponse(ctx context.Context, body io.Reader) types.ResponseChain {
	chain, sender := types.NewResponseChain(10)
	go func() {
		defer close(sender)
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			line := scanner.Bytes()

			// 跳过空行
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}

			// 解析流式数据块
			chunk, err := k.converter.ConvertStreamChunk(ctx, line)
			if err != nil {
				sender.Send(ctx, &types.Response{
					Error: &types.ResponseError{
						Code:    stringPtr("STREAM_PARSE_ERROR"),
						Message: fmt.Sprintf("failed to parse stream chunk: %v", err),
					},
				})
				return
			}

			// 跳过空块（如 [DONE] 标记）
			if chunk == nil {
				continue
			}

			sender.Send(ctx, chunk)

			// 如果是最终块，结束流
			if chunk.Done {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			sender.Send(ctx, &types.Response{
				Error: &types.ResponseError{
					Code:    stringPtr("STREAM_READ_ERROR"),
					Message: fmt.Sprintf("failed to read stream: %v", err),
				},
			})
			return
		}

		// 发送流结束标记
		sender.Send(ctx, &types.Response{
			ID:        generateResponseID(),
			Object:    "chat.completion.chunk",
			Created:   time.Now().Unix(),
			Model:     "kiro-codewhisperer",
			Timestamp: time.Now(),
			Done:      true,
			IsPartial: false,
			Choices: []types.Choice{
				{
					Index:        0,
					FinishReason: stringPtr("stop"),
				},
			},
		})
	}()
	return chain
}

// parseStreamChunk 解析单个流式响应数据块（包内私有函数）
func parseStreamChunk(data []byte) (*types.Response, error) {
	// 去除 SSE 前缀 "data: "
	chunk := bytes.TrimPrefix(data, []byte("data: "))
	chunk = bytes.TrimSpace(chunk)

	// 跳过空行和结束标记
	if len(chunk) == 0 || string(chunk) == "[DONE]" {
		return nil, nil
	}

	// 解析 Kiro 流式响应块
	var kiroResp KiroResponse
	if err := json.Unmarshal(chunk, &kiroResp); err != nil {
		return nil, fmt.Errorf("failed to parse stream chunk: %w", err)
	}

	// 转换为标准流式响应格式
	response := &types.Response{
		ID:        generateResponseID(),
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Model:     "kiro-codewhisperer",
		Timestamp: time.Now(),
		Done:      false,
		IsPartial: true,
	}

	// 处理错误
	if kiroResp.Error != nil {
		response.Error = &types.ResponseError{
			Code:    &kiroResp.Error.Code,
			Message: kiroResp.Error.Message,
		}
		response.Done = true
		return response, nil
	}

	// 构建 delta 内容
	content := kiroResp.Content
	if kiroResp.ThinkingContent != "" {
		content = fmt.Sprintf("%s\n\n[Thinking] %s", content, kiroResp.ThinkingContent)
	}

	response.Choices = []types.Choice{
		{
			Index: 0,
			Delta: types.Message{
				Role:    types.RoleAssistant,
				Content: content,
			},
		},
	}

	return response, nil
}

// validateRequest validates the incoming request
func validateRequest(req types.Request) error {
	if len(req.Messages) == 0 {
		return fmt.Errorf("request must contain at least one message")
	}
	return nil
}

// generateResponseID generates a unique response ID using crypto/rand
func generateResponseID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// 降级：使用时间戳
		return fmt.Sprintf("kiro_%d", time.Now().UnixNano())
	}
	return "kiro_" + hex.EncodeToString(b)
}

// generateUUID generates a UUID v4 format string using crypto/rand
func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	// 设置 UUID v4 版本位和变体位
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}


