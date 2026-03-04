package providerclient

import (
	"context"

	"github.com/nomand-zc/token101/provider-client/types"
)

type Model interface {
	GenerateContent(ctx context.Context, account types.Account, req types.Request) (*types.Response, error)
	GenerateContentStream(ctx context.Context, account types.Account, req types.Request) (types.ResponseChain, error)
}
