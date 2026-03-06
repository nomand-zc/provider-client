package auth

import (
	"context"

	"github.com/nomand-zc/provider-client/credentials"
	"github.com/nomand-zc/provider-client/providers"
)

// VerifyQuota 验证配额是否足够
func VerifyQuota(provider providers.Provider, creds credentials.Credentials) bool {
	usage, err := provider.GetUsage(context.Background(), creds)
	if err != nil {
		return false
	}
	if len(usage) == 0 {
		return false
	}

	for _, u := range usage {
		if u.IsTriggered() {
			return false
		}
	}
	return true
}
