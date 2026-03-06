package token

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nomand-zc/provider-client/cli/internal/auth"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
	kiroprovider "github.com/nomand-zc/provider-client/providers/kiro"
)

var (
	defaultRefresher refresher
)

// refresher 持有 token refresh 命令的参数
type refresher struct {
	credFile     string
	providerName string
	provider     providers.Provider
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
	cmd.Flags().StringVarP(&r.providerName, "provider", "p", "kiro", fmt.Sprintf("provider 名称，支持：%v（必填）", "kiro"))
	_ = cmd.MarkFlagRequired("file")

	return cmd
}

// run 执行 token refresh 逻辑
func (r *refresher) run() error {
	switch r.providerName {
	case "kiro":
		r.provider = kiroprovider.NewProvider()
	default:
		return fmt.Errorf("不支持的 provider: %q，支持的 provider 列表：%v", r.provider, "kiro")
	}
	info, err := os.Stat(r.credFile)
	if err != nil {
		return fmt.Errorf("访问路径失败: %w", err)
	}

	if info.IsDir() {
		return r.runDir(r.credFile)
	}
	return r.runFile(r.credFile)
}

// runDir 递归处理目录下所有 .json 文件
func (r *refresher) runDir(dir string) error {
	var successCount, failureCount int

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("遍历路径 %q 失败: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".json") {
			return nil
		}
		if err := r.runFile(path); err != nil {
			failureCount++
			return nil
		}
		successCount++
		return nil
	})

	log.Infof("刷新完成！总凭证数量: %d, 成功：%d，失败：%d\n", successCount+failureCount, successCount, failureCount)
	return err
}

// runFile 读取、刷新并回写单个凭证 JSON 文件
func (r *refresher) runFile(filePath string) error {
	// 读取凭证文件
	creds, err := auth.LoadCredentials(r.providerName, filePath)
	if err != nil {
		return err
	}

	newCreds, err := r.provider.Refresh(context.Background(), creds)
	if err != nil {
		return fmt.Errorf("刷新凭证失败: %w", err)
	}

	// 将刷新后的凭证写回文件
	if err := auth.SaveCredentials(newCreds, filePath); err != nil {
		return fmt.Errorf("保存凭证失败: %w", err)
	}
	return nil
}
