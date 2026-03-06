package common

import (
	"fmt"

	"github.com/nomand-zc/provider-client/credentials"
	kirocreds "github.com/nomand-zc/provider-client/credentials/kiro"
)

// BuildCredentials 根据 provider 名称和原始数据构建凭证对象
// 这是一个公共工具函数，供所有需要构建凭证的命令使用
func BuildCredentials(providerName string, raw []byte) (credentials.Credentials, error) {
	var creds credentials.Credentials
	var err error

	switch providerName {
	case "kiro":
		creds = kirocreds.NewCredentials(raw)
	default:
		return nil, fmt.Errorf("不支持的 provider: %q，支持的 provider 列表：%v", providerName, "kiro")
	}

	// 验证凭证，但允许过期凭证（ErrExpiresAtExpired 错误不视为失败）
	if err = creds.Validate(); err != nil && err != credentials.ErrExpiresAtExpired {
		return creds, fmt.Errorf("验证凭证失败: %w", err)
	}

	return creds, nil
}
