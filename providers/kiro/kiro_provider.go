package kiro

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/google/uuid"
	"github.com/nomand-zc/provider-client/credentials"
	kirocreds "github.com/nomand-zc/provider-client/credentials/kiro"
	"github.com/nomand-zc/provider-client/httpclient"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
	"github.com/nomand-zc/provider-client/providers/kiro/converter"
	"github.com/nomand-zc/provider-client/providers/kiro/converter/parser"
	"github.com/nomand-zc/provider-client/queue"
)

const (
	providerName     = "kiro"
	defaultQueueSize = 100
	// 事件流解码 payload 缓冲区大小
	defaultPayloadBufSize = 10 * 1024
)

type kiroProvider struct {
	httpClient httpclient.HTTPClient
	options    *Options
}

// NewProvider creates a new kiro provider.
func NewProvider(opts ...Option) *kiroProvider {
	options := &defaultOptions
	for _, opt := range opts {
		opt(options)
	}
	return &kiroProvider{
		options:    options,
		httpClient: httpclient.New(),
	}
}

// Name returns the name of the provider.
func (p *kiroProvider) Name() string {
	return providerName
}

// GenerateContent generates content.
func (p *kiroProvider) GenerateContent(ctx context.Context, creds credentials.Credentials, 
	req providers.Request) (*providers.Response, error) {
	reader, err := p.GenerateContentStream(ctx, creds, req)
	if err != nil {
		return nil, err
	}
	var resp *providers.Response
	for {
		// TODO: 读取流信息，然后拼接成一条完整的响应
		item, err := reader.Read(ctx)
		
	}
	return resp, nil
}

// GenerateContentStream generates content in a stream.
func (p *kiroProvider) GenerateContentStream(ctx context.Context, creds credentials.Credentials, 
	req providers.Request) (queue.Reader[*providers.Response], error){
	// 1. 初始化调用上下文
	ctx, inv := providers.EnsureInvocationContext(ctx)
	inputTokens, err := p.options.tokenConter.CountTokensRange(ctx, req.Messages, 0, len(req.Messages))
	if err != nil {
		// token 计算失败
		return nil, fmt.Errorf("failed to calculate tokens: %w", err)
	}
	inv.Usage.PromptTokens = inputTokens
	inv.ID = uuid.NewString()

	// 2. 构建请求信息
	kiroCreds := creds.(*kirocreds.Credentials)
	url := fmt.Sprintf(p.options.url, kiroCreds.Region)
	cwReq := converter.ConvertRequest(ctx, req)
	if kiroCreds.AuthMethod == kirocreds.AuthMethodSocial && kiroCreds.ProfileArn != "" {
		cwReq.ProfileArn = kiroCreds.ProfileArn
	}
	cwReqBody, err := json.Marshal(cwReq)
	if err != nil {
		return nil, err
	}
	log.Debugf("[kiroProvider.GenerateContentStream] kiro request: %s", string(cwReqBody))
	request, err := http.NewRequest("POST", url, bytes.NewReader(cwReqBody))
	if err != nil {
		return nil, err
	}
	for key, value := range p.options.headerBuilder() {
		request.Header.Set(key, value)
	}
	// request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", kiroCreds.AccessToken))
	request.Header.Set("amz-sdk-invocation-id", inv.ID)

	// 3. 发送请求, 并检查状态码
	resp, err := p.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, &providers.HTTPError{
			ErrorType:       providers.ErrorTypeForbidden,
			ErrorCode:       resp.StatusCode,
			Message:         fmt.Sprintf("HTTP status code: %d", resp.StatusCode),
			RawStatusCode:   resp.StatusCode,
		}
	}

	// 4. 解码流式事件内容
	return p.handlerStreamEvent(ctx, inv, resp.Body), nil
}

func (p *kiroProvider) handlerStreamEvent(ctx context.Context, inv *providers.Invocation, 
	respBody io.ReadCloser) queue.Reader[*providers.Response] {
	chainQueue := queue.NewChanQueue[*providers.Response](defaultQueueSize)
	decoder := eventstream.NewDecoder()
	payloadBuf := make([]byte, defaultPayloadBufSize)

	go func() {
		defer chainQueue.Close()
		defer respBody.Close()

		// 收集用量统计信息，最后随 stop 事件一起发送
		var collectedUsage providers.Usage
		collectedUsage.PromptTokens = inv.Usage.PromptTokens
		var firstErr error

		for {
			// 重置 payloadBuf 以复用底层数组
			payloadBuf = payloadBuf[0:0]
			e, err := decoder.Decode(respBody, payloadBuf)
			if err != nil {
				if !errors.Is(err, queue.ErrQueueClosed) {
					firstErr = err
					log.Errorf("kiro stream decode error: %v", err)
				}
				break
			}

			msg := parser.StreamMessage(e)
			result, err := converter.ConvertResponse(ctx, &msg)
			if err != nil || result == nil {
				continue
			}

			// 如果是用量统计信息事件，收集起来而不直接发送
			if msg.IsMetricMessage() && result.Usage != nil {
				collectedUsage.Credit = result.Usage.Credit
			}

			if msg.ShouldSendMessage() {
				// 修复ID
				result.ID = inv.ID
			chainQueue.Write(ctx, result)
			}
		}

		// 发送带有 usage 信息的 stop 响应
		collectedUsage.TotalTokens = collectedUsage.PromptTokens + collectedUsage.CompletionTokens
		finishReason := "stop"
		finalResp := &providers.Response{
			ID:        inv.ID,
			Object:    "chat.completion.chunk",
			Created:   time.Now().Unix(),
			Timestamp: time.Now(),
			Done:      true,
			IsPartial: false,
			Usage:     &collectedUsage,
			Choices: []providers.Choice{
				{
					FinishReason: &finishReason,
				},
			},
		}
		if firstErr != nil {
			finalResp.Error = &providers.ResponseError{
				Message: firstErr.Error(),
				Type:    "stream_parse_error",
			}
		}
		chainQueue.Write(ctx, finalResp)
	}()

	return chainQueue
}
