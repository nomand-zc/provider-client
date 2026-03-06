package credentials

import (
	"time"

	"github.com/nomand-zc/provider-client/utils"
)

// Credentials is an interface for credentials used to authenticate with a provider. It includes methods for refreshing credentials, retrieving access and refresh tokens, checking expiration, and converting credentials to a map format for storage or transmission.
type Credentials interface {
	// Validate validates the credentials.
	Validate() error
	// GetAccessToken returns the access token.
	GetAccessToken() string
	// GetRefreshToken returns the refresh token.
	GetRefreshToken() string
	// GetExpiresAt returns the expiration time of the credentials.
	GetExpiresAt() *time.Time
	// IsExpired returns true if the credentials are expired.
	IsExpired() bool
	// ToMap converts the credentials to a map format for storage or transmission.
	ToMap() map[string]any
}

// GetValue gets the value of the given key from the credentials. It returns the value and a boolean indicating whether the key exists and is of the correct type.
func GetValue[V any] (creds Credentials, key string) (V, bool) {
	return utils.GetMapValue[string, V](creds.ToMap(), key)
}