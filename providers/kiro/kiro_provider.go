package kiro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/google/uuid"
	"github.com/nomand-zc/provider-client/credentials"
	kirocreds "github.com/nomand-zc/provider-client/credentials/kiro"
	"github.com/nomand-zc/provider-client/httpclient"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
	"github.com/nomand-zc/provider-client/providers/kiro/converter"
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
	return nil, nil
}

// GenerateContentStream generates content in a stream.
func (p *kiroProvider) GenerateContentStream(ctx context.Context, creds credentials.Credentials, 
	req providers.Request) (queue.Reader[*providers.Response], error){
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
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", kiroCreds.AccessToken))
	request.Header.Set("amz-sdk-invocation-id", uuid.NewString())

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

	chainQueue := queue.NewChanQueue[*providers.Response](defaultQueueSize)
	decoder := eventstream.NewDecoder()
	payloadBuf := make([]byte, defaultPayloadBufSize)

	go func() {
		defer chainQueue.Close()
		defer resp.Body.Close()
		for {
			// 重置 payloadBuf 以复用底层数组
			payloadBuf = payloadBuf[0:0]
			msg, err := decoder.Decode(resp.Body, payloadBuf)
			if err != nil {
				if err != io.EOF {
					log.Errorf("kiro stream decode error: %v", err)
				}
				return
			}

			result, err := converter.ConvertResponse(ctx, &msg)
			if err != nil || result == nil {
				continue
			}
			chainQueue.Write(result)
		}
	}()

	return chainQueue, nil
}
