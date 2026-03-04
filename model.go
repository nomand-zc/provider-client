package providerclient

import (
	"context"

	"github.com/nomand-zc/token101/provider-client/types"
)

type Model interface {
	GenerateContent(ctx context.Context, creds any, req types.Request) (*types.Response, error)
	GenerateContentStream(ctx context.Context, creds any, req types.Request) (types.ResponseChain, error)
}
