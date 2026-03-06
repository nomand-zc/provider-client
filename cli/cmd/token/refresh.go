package token

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nomand-zc/provider-client/credentials"
	kirocreds "github.com/nomand-zc/provider-client/credentials/kiro"
	"github.com/nomand-zc/provider-client/providers"
	kiroprovider "github.com/nomand-zc/provider-client/providers/kiro"
)

var (
	defaultRefresher refresher
)

// refresher 持有 token refresh 命令的参数
type refresher struct {
	credFile string
	provider string
}

func (r refresher) cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "刷新 token 凭证",
		Long: `刷新指定 provider 的 token 凭证，并将刷新后的凭证回写到指定的 JSON 文件中。

支持的 provider：
  - kiro

示例：
  provider-client token refresh --file /path/to/credentials.json --provider kiro`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return r.run()
		},
	}

	cmd.Flags().StringVarP(&r.credFile, "file", "f", "", "凭证 JSON 文件路径（必填）")
	cmd.Flags().StringVarP(&r.provider, "provider", "p", "", fmt.Sprintf("provider 名称，支持：%v（必填）", "kiro"))
	_ = cmd.MarkFlagRequired("file")
	_ = cmd.MarkFlagRequired("provider")

	return cmd
}

// run 执行 token refresh 逻辑
func (r *refresher) run() error {
	// 读取凭证文件
	fileData, err := os.ReadFile(r.credFile)
	if err != nil {
		return fmt.Errorf("读取凭证文件失败: %w", err)
	}

	var (
		provider providers.Provider
		creds    credentials.Credentials
	)

	switch r.provider {
	case "kiro":
		provider = kiroprovider.NewProvider()
		creds = kirocreds.NewCredentials(fileData)
	default:
		return fmt.Errorf("不支持的 provider: %q，支持的 provider 列表：%v", r.provider, "kiro")
	}

	if err := creds.Validate(); err != nil && err != credentials.ErrExpiresAtExpired {
		return fmt.Errorf("验证凭证失败: %w", err)
	}

	newCreds, err := provider.Refresh(context.Background(), creds)
	if err != nil {
		return fmt.Errorf("刷新凭证失败: %w", err)
	}

	// 将刷新后的凭证写回文件
	credsJSON, err := json.Marshal(newCreds)
	if err != nil {
		return fmt.Errorf("序列化凭证失败: %w", err)
	}
	if err := os.WriteFile(r.credFile, credsJSON, 0600); err != nil {
		return fmt.Errorf("写入凭证文件失败: %w", err)
	}
	return nil
}
