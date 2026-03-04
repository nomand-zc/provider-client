package kiro

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/gg/gcond"
	providerclient "github.com/nomand-zc/token101/provider-client"
	"github.com/nomand-zc/token101/provider-client/client"
	"github.com/nomand-zc/token101/provider-client/log"
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
func (k *kiro) buildHTTPRequest(ctx context.Context, rawCreds any, req types.Request) (*http.Request, error) {
	// 将标准请求序列化为 Kiro 格式的 JSON 字节切片（凭证用于注入 profileArn）
	body, err := k.converter.ConvertRequest(ctx, rawCreds, req)
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] failed to convert request: %v", err)
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	creds, err := extractKiroCredentials(rawCreds)
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] failed to extract credentials: %v", err)
		return nil, fmt.Errorf("failed to extract credentials: %w", err)
	}
	// 构建请求 URL
	url := k.buildURL(*creds)
	log.DebugfContext(ctx, "[kiro] buildHTTPRequest url=%s region=%s bodyLen=%d", url, gcond.If(creds.Region != "", creds.Region, k.options.defaultRegion), len(body))

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] failed to create HTTP request: %v", err)
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// 构建并设置请求头
	headers, err := k.buildHeaders(*creds)
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] failed to build headers: %v", err)
		return nil, fmt.Errorf("failed to build headers: %w", err)
	}
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	return httpReq, nil
}

// GenerateContent implements the Model interface for synchronous content generation
func (k *kiro) GenerateContent(ctx context.Context, creds any, req types.Request) (*types.Response, error) {
	log.DebugfContext(ctx, "[kiro] GenerateContent start, msgCount=%d", len(req.Messages))

	// 验证请求
	if err := validateRequest(req); err != nil {
		log.ErrorfContext(ctx, "[kiro] GenerateContent invalid request: %v", err)
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 构建 HTTP 请求（含 body、URL 和 header）
	httpReq, err := k.buildHTTPRequest(ctx, creds, req)
	if err != nil {
		return nil, err
	}

	// 发送 HTTP 请求
	log.DebugfContext(ctx, "[kiro] GenerateContent sending HTTP request")
	resp, err := k.httpClient.Do(httpReq)
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] GenerateContent failed to send request: %v", err)
		return nil, fmt.Errorf("failed to send request to Kiro API: %w", err)
	}

	// 读取并关闭响应体
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] GenerateContent failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	log.DebugfContext(ctx, "[kiro] GenerateContent got response statusCode=%d contentType=%s bodyLen=%d",
		resp.StatusCode, resp.Header.Get("Content-Type"), len(respBody))

	// 检查响应状态码，非 2xx 时返回归一化的 HTTPError
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.WarnfContext(ctx, "[kiro] GenerateContent non-2xx response statusCode=%d body=%s",
			resp.StatusCode, string(respBody))
		return nil, NewHTTPError(resp.StatusCode, respBody)
	}

	// Kiro API 始终返回 AWS 事件流二进制格式（application/vnd.amazon.eventstream）
	// 将所有事件帧聚合为一个完整响应
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/vnd.amazon.eventstream") {
		log.DebugfContext(ctx, "[kiro] GenerateContent parsing AWS event stream")
		kiroResp, err := ParseAWSEventStream(respBody)
		if err != nil {
			log.ErrorfContext(ctx, "[kiro] GenerateContent failed to parse AWS event stream: %v", err)
			return nil, fmt.Errorf("failed to parse AWS event stream: %w", err)
		}
		log.DebugfContext(ctx, "[kiro] GenerateContent AWS event stream parsed, contentLen=%d toolCalls=%d",
			len(kiroResp.Content), len(kiroResp.ToolCalls))
		return convertKiroResponseToStandard(kiroResp), nil
	}

	// 兜底：尝试普通 JSON 解析
	log.DebugfContext(ctx, "[kiro] GenerateContent fallback to JSON parse, contentType=%s", contentType)
	response, err := k.converter.ConvertResponse(ctx, respBody)
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] GenerateContent failed to convert response: %v", err)
		return nil, fmt.Errorf("failed to convert response: %w", err)
	}

	log.DebugfContext(ctx, "[kiro] GenerateContent done")
	return response, nil
}

// GenerateContentStream implements the Model interface for streaming content generation
func (k *kiro) GenerateContentStream(ctx context.Context, creds any, req types.Request) (types.ResponseChain, error) {
	log.DebugfContext(ctx, "[kiro] GenerateContentStream start, msgCount=%d", len(req.Messages))

	// 验证请求
	if err := validateRequest(req); err != nil {
		log.ErrorfContext(ctx, "[kiro] GenerateContentStream invalid request: %v", err)
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 设置流式模式
	req.GenerationConfig.Stream = true

	// 构建 HTTP 请求（含 body、URL 和 header）
	httpReq, err := k.buildHTTPRequest(ctx, creds, req)
	if err != nil {
		return nil, err
	}

	// 发送 HTTP 请求
	log.DebugfContext(ctx, "[kiro] GenerateContentStream sending HTTP request")
	resp, err := k.httpClient.Do(httpReq)
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] GenerateContentStream failed to send request: %v", err)
		return nil, fmt.Errorf("failed to send request to Kiro API: %w", err)
	}
	log.DebugfContext(ctx, "[kiro] GenerateContentStream got response statusCode=%d contentType=%s",
		resp.StatusCode, resp.Header.Get("Content-Type"))

	// 检查响应状态码，非 2xx 时读取 body 后返回归一化的 HTTPError
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		log.WarnfContext(ctx, "[kiro] GenerateContentStream non-2xx response statusCode=%d body=%s",
			resp.StatusCode, string(errBody))
		return nil, NewHTTPError(resp.StatusCode, errBody)
	}

	// 创建响应 channel
	chain, sender := types.NewResponseChain(10)

	// 在 goroutine 中处理流式响应
	go func() {
		defer close(sender)
		defer resp.Body.Close()

		// 根据 Content-Type 选择响应处理方式
		contentType := resp.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "application/vnd.amazon.eventstream") {
			log.DebugfContext(ctx, "[kiro] GenerateContentStream handling AWS event stream")
			// Kiro API 返回 AWS 事件流二进制格式，逐帧发送
			for response := range k.handleAWSEventStreamResponse(ctx, resp.Body) {
				if !sender.Send(ctx, response) {
					return
				}
				if response.Done || response.Error != nil {
					if response.Error != nil {
						log.ErrorfContext(ctx, "[kiro] GenerateContentStream AWS event stream error: code=%v msg=%s",
							response.Error.Code, response.Error.Message)
					}
					return
				}
			}
			log.DebugfContext(ctx, "[kiro] GenerateContentStream AWS event stream done")
			return
		}

		if !strings.HasPrefix(contentType, "text/event-stream") && !strings.HasPrefix(contentType, "application/x-ndjson") {
			log.DebugfContext(ctx, "[kiro] GenerateContentStream handling non-stream response, contentType=%s", contentType)
			response := k.handleNonStreamResponse(ctx, resp.Body)
			if response.Error != nil {
				log.ErrorfContext(ctx, "[kiro] GenerateContentStream non-stream response error: code=%v msg=%s",
					response.Error.Code, response.Error.Message)
			}
			sender.Send(ctx, response)
			return
		}

		log.DebugfContext(ctx, "[kiro] GenerateContentStream handling SSE stream, contentType=%s", contentType)
		for response := range k.handleSSEStreamResponse(ctx, resp.Body) {
			if !sender.Send(ctx, response) {
				return
			}
			if response.Done || response.Error != nil {
				if response.Error != nil {
					log.ErrorfContext(ctx, "[kiro] GenerateContentStream SSE stream error: code=%v msg=%s",
						response.Error.Code, response.Error.Message)
				}
				return
			}
		}
		log.DebugfContext(ctx, "[kiro] GenerateContentStream SSE stream done")
	}()

	return chain, nil
}

// handleAWSEventStreamResponse 处理 AWS 事件流二进制格式响应（application/vnd.amazon.eventstream）
// 读取完整响应体后逐帧解析，每个内容帧作为一个流式响应块发送
func (k *kiro) handleAWSEventStreamResponse(ctx context.Context, body io.Reader) types.ResponseChain {
	chain, sender := types.NewResponseChain(10)
	go func() {
		defer close(sender)

		// 读取完整响应体（AWS 事件流需要完整数据才能解析帧边界）
		data, err := io.ReadAll(body)
		if err != nil {
			log.ErrorfContext(ctx, "[kiro] handleAWSEventStreamResponse failed to read body: %v", err)
			sender.Send(ctx, &types.Response{
				Error: &types.ResponseError{
					Code:    stringPtr("READ_ERROR"),
					Message: fmt.Sprintf("failed to read AWS event stream body: %v", err),
				},
			})
			return
		}
		log.DebugfContext(ctx, "[kiro] handleAWSEventStreamResponse read body dataLen=%d", len(data))

		// 解析为流式响应块列表
		chunks, err := ParseAWSEventStreamChunks(data)
		if err != nil {
			log.ErrorfContext(ctx, "[kiro] handleAWSEventStreamResponse failed to parse event stream: %v", err)
			sender.Send(ctx, &types.Response{
				Error: &types.ResponseError{
					Code:    stringPtr("PARSE_ERROR"),
					Message: fmt.Sprintf("failed to parse AWS event stream: %v", err),
				},
			})
			return
		}
		log.DebugfContext(ctx, "[kiro] handleAWSEventStreamResponse parsed chunks=%d", len(chunks))

		// 逐块发送内容帧
		for _, kiroResp := range chunks {
			resp := convertKiroResponseToStreamChunk(kiroResp)
			if !sender.Send(ctx, resp) {
				return
			}
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

// handleNonStreamResponse 处理非流式响应：读取 body 并转换为标准响应格式后返回
func (k *kiro) handleNonStreamResponse(ctx context.Context, body io.Reader) *types.Response {
	respBody, err := io.ReadAll(body)
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] handleNonStreamResponse failed to read body: %v", err)
		return &types.Response{
			Error: &types.ResponseError{
				Code:    stringPtr("READ_ERROR"),
				Message: fmt.Sprintf("failed to read response body: %v", err),
			},
		}
	}
	log.DebugfContext(ctx, "[kiro] handleNonStreamResponse bodyLen=%d", len(respBody))
	response, err := k.converter.ConvertResponse(ctx, respBody)
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] handleNonStreamResponse failed to parse response: %v, body=%s", err, string(respBody))
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
				log.ErrorfContext(ctx, "[kiro] handleSSEStreamResponse failed to parse chunk: %v, line=%s", err, string(line))
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
				log.DebugfContext(ctx, "[kiro] handleSSEStreamResponse stream done")
				return
			}
		}

		if err := scanner.Err(); err != nil {
			log.ErrorfContext(ctx, "[kiro] handleSSEStreamResponse scanner error: %v", err)
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

// extractKiroCredentials 从 any 类型的凭证中提取 *Credentials
// 支持 *Credentials 直接传入，或 []byte（JSON 格式）两种形式
func extractKiroCredentials(rawCreds any) (*Credentials, error) {
	switch v := rawCreds.(type) {
	case *Credentials:
		return v, nil
	case Credentials:
		return &v, nil
	case []byte:
		return ExtractCredentials(v)
	default:
		return nil, fmt.Errorf("unsupported credentials type: %T", rawCreds)
	}
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
