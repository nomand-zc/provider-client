package kiro

import (
	"context"

	"github.com/nomand-zc/provider-client/credentials"
	"github.com/nomand-zc/provider-client/httpclient"
	"github.com/nomand-zc/provider-client/providers"
	"github.com/nomand-zc/provider-client/queue"
)

const (
	providerName = "kiro"
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
		return nil, nil
}

// buildURL 构建 Kiro API 请求 URL，优先使用凭证中的 region
// func (k *kiroProvider) buildURL(creds Credentials) string {
// 	region := gcond.If(creds.Region != "", creds.Region, k.options.defaultRegion)
// 	return fmt.Sprintf(k.options.url, region)
// }

// // buildHeaders 根据凭证和配置选项构建 Kiro API 所需的 HTTP 请求头
// func (k *kiroProvider) buildHeaders(creds Credentials) (map[string]string, error) {
// 	// 以 defaultOptions.headers 为基础构建 header map
// 	headers := make(map[string]string, len(defaultOptions.headers)+1)
// 	maps.Copy(headers, defaultOptions.headers)

// 	// 设置认证头
// 	headers["Authorization"] = fmt.Sprintf("Bearer %s", creds.AccessToken)

// 	// 设置每次请求唯一的调用 ID（UUID v4 格式）
// 	headers["amz-sdk-invocation-id"] = generateUUID()

// 	// 合并 options.headers 中的自定义 header（可覆盖默认值）
// 	maps.Copy(headers, k.options.headers)

// 	log.DebugfContext(context.Background(), "[kiro] buildHeaders headers=%v", headers)

// 	return headers, nil
// }
