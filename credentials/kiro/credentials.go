package kiro

import (
	"encoding/json"
	"time"

	"github.com/nomand-zc/provider-client/credentials"
)

const (
	AuthMethodSocial = "social"
)

type Credentials struct {
	AccessToken  string     `json:"accessToken"`
	RefreshToken string     `json:"refreshToken,omitempty"`
	ProfileArn   string     `json:"profileArn,omitempty"`
	AuthMethod   string     `json:"authMethod,omitempty"`
	Provider     string     `json:"provider,omitempty"`
	Region       string     `json:"region,omitempty"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`

	// IDC (Builder ID) 模式专用字段
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	IDCRegion    string `json:"idcRegion,omitempty"` // IDC 模式使用的区域，对应 AIClient-2-API 的 idcRegion

	raw map[string]any `json:"-"` // 原始凭证数据，保留所有字段以便刷新时使用
}

// NewCredentials 创建一个新的凭据实例
// 支持传入 JSON 字符串或 []byte，解析失败时返回 nil
func NewCredentials[T string | []byte](raw T) *Credentials {
	var creds Credentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return nil
	}
	return &creds
}

// Clone 克隆凭据实例
func (c *Credentials) Clone() *Credentials {
	clone := *c
	if c.ExpiresAt != nil {
		t := *c.ExpiresAt
		clone.ExpiresAt = &t
	}
	return &clone
}

// Validate 校验凭据的格式有效性
// 仅校验格式，不校验凭证是否过期
func (c *Credentials) Validate() error {
	if c == nil {
		return credentials.ErrCredentialsEmpty
	}
	if c.AccessToken == "" {
		return credentials.ErrAccessTokenEmpty
	}
	if c.RefreshToken == "" {
		return credentials.ErrRefreshTokenEmpty
	}
	if c.ProfileArn == "" {
		return credentials.ErrProfileArnEmpty
	}
	if c.ExpiresAt == nil {
		return credentials.ErrExpiresAtEmpty
	}
	if c.AuthMethod == "" {
		return credentials.ErrAuthMethodEmpty
	}
	if c.Region == "" {
		return credentials.ErrRegionEmpty
	}

	if c.AuthMethod == AuthMethodSocial {
		return nil
	}
	// IDC 模式需要额外验证的字段
	if c.IDCRegion == "" {
		return credentials.ErrIDCRegionEmpty
	}
	if c.ClientID == "" {
		return credentials.ErrClientIDEmpty
	}
	if c.ClientSecret == "" {
		return credentials.ErrClientSecretEmpty
	}

	return nil
}

// GetAccessToken 返回访问令牌
func (c *Credentials) GetAccessToken() string {
	return c.AccessToken
}

// GetRefreshToken 返回刷新令牌
func (c *Credentials) GetRefreshToken() string {
	return c.RefreshToken
}

// GetExpiresAt 返回过期时间
func (c *Credentials) GetExpiresAt() *time.Time {
	return c.ExpiresAt
}

// IsExpired 检查凭据是否过期
func (c *Credentials) IsExpired() bool {
	if c.ExpiresAt == nil {
		return true
	}
	return time.Now().After(*c.ExpiresAt)
}

// ToMap 将凭据转换为map格式
func (c *Credentials) ToMap() map[string]any {
	if c.raw != nil {
		return c.raw
	}
	c.raw = map[string]any{
		"accessToken":  c.AccessToken,
		"refreshToken": c.RefreshToken,
		"profileArn":   c.ProfileArn,
		"authMethod":   c.AuthMethod,
		"provider":     c.Provider,
		"region":       c.Region,
		"expiresAt":    c.ExpiresAt,
		"clientId":     c.ClientID,
		"clientSecret": c.ClientSecret,
		"idcRegion":    c.IDCRegion,
	}
	return c.raw
}
