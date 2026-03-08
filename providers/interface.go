package providers

import (
	"context"

	"github.com/nomand-zc/provider-client/credentials"
	"github.com/nomand-zc/provider-client/queue"
	"github.com/nomand-zc/provider-client/usagerule"
)

// Provider is an interface for a provider. It includes methods for generating content and generating content in a stream. The GenerateContent method takes a context, credentials, and a request, and returns a response or an error. The GenerateContentStream method takes the same parameters but returns a ResponseChain for streaming responses.
type Model interface {
	// GenerateContent generates content.
	GenerateContent(ctx context.Context, creds credentials.Credentials, req Request) (*Response, error)
	// GenerateContentStream generates content in a stream.
	GenerateContentStream(ctx context.Context, creds credentials.Credentials, req Request) (queue.Reader[*Response], error)
}

// TokenRefresher is an interface for refreshing tokens. It includes a method for refreshing tokens, which takes a context and credentials, and returns refreshed credentials or an error.
type TokenRefresher interface {
	// Refresh refreshes tokens.
	Refresh(ctx context.Context, creds credentials.Credentials) (credentials.Credentials, error)
}

// UsageLimiter is an interface for limiting usage. It includes methods for listing models and getting usage, both of which take a context and credentials, and return a list of models or the usage count, respectively, or an error.
type UsageLimiter interface {
	// Models lists models.
	// 默认支持的模型列表
	Models(ctx context.Context) ([]string, error)

	// ListModels lists models.
	// 获取当前凭证支持的模型列表
	ListModels(ctx context.Context, creds credentials.Credentials) ([]string, error)

	// GetUsage gets usage.
	GetUsage(ctx context.Context, creds credentials.Credentials) ([]*usagerule.UsageRule, error)
}

// Provider is an interface for a provider. It includes methods for generating content, generating content in a stream, refreshing tokens, and limiting usage.
type Provider interface {
	Name() string
	Model
	TokenRefresher
	UsageLimiter
}