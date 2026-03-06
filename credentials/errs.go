package credentials

import "errors"

var (
	ErrCredentialsEmpty = errors.New("credentials is empty")
	// ErrAccessTokenEmpty 表示访问令牌为空的错误
	ErrAccessTokenEmpty = errors.New("access token is empty")
	// ErrRefreshTokenEmpty 表示刷新令牌为空的错误
	ErrRefreshTokenEmpty = errors.New("refresh token is empty")
	// ErrProfileArnEmpty 表示 profile arn 为空的错误
	ErrProfileArnEmpty = errors.New("profile arn is empty")
	// ErrExpiresAtEmpty 表示过期时间为空的错误
	ErrExpiresAtEmpty = errors.New("expires at is empty")
	// ErrAuthMethodEmpty 表示认证方式为空的错误
	ErrAuthMethodEmpty = errors.New("auth method is empty")
	// ErrRegionEmpty 表示区域为空的错误
	ErrRegionEmpty = errors.New("region is empty")
	// ErrIDCRegionEmpty 表示 IDC 区域为空的错误
	ErrIDCRegionEmpty = errors.New("idc region is empty")
	// ErrClientIDEmpty 表示客户端 ID 为空的错误
	ErrClientIDEmpty = errors.New("client id is empty")
	// ErrClientSecretEmpty 表示客户端密钥为空的错误
	ErrClientSecretEmpty = errors.New("client secret is empty")
	// ErrExpiresAtExpired 表示过期时间已过期的错误
	ErrExpiresAtExpired = errors.New("expires at is expired")
)
