package factory

import (
	"fmt"

	"github.com/nomand-zc/provider-client/providers"
	kiroprovider "github.com/nomand-zc/provider-client/providers/kiro"
)

// SupportedProviders 支持的 provider 列表
var SupportedProviders = []string{"kiro"}

// NewProvider 根据名称创建对应的 provider 实例
func NewProvider(name string) (providers.Provider, error) {
	switch name {
	case "kiro":
		return kiroprovider.NewProvider(), nil
	default:
		return nil, fmt.Errorf("不支持的 provider: %q，支持的 provider 列表：%v", name, SupportedProviders)
	}
}
