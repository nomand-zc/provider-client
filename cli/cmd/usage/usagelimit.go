package usage

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
	defaultUsageViewer usageViewer
)

// usageViewer 持有 usage view 命令的参数
type usageViewer struct {
	credFile     string
	providerName string
	provider     providers.Provider
}

func (u usageViewer) cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "查看凭证用量信息",
		Long: `查看指定 provider 的凭证用量信息，包括日限额、月限额、已使用量等。

支持的 provider：
  - kiro

示例：
  provider-client usage view --file /path/to/credentials.json --provider kiro`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return u.run()
		},
	}

	cmd.Flags().StringVarP(&u.credFile, "file", "f", "", "凭证 JSON 文件路径（必填）")
	cmd.Flags().StringVarP(&u.providerName, "provider", "p", "kiro", fmt.Sprintf("provider 名称，支持：%v（必填）", "kiro"))
	_ = cmd.MarkFlagRequired("file")

	return cmd
}

// run 执行 usage view 逻辑
func (u *usageViewer) run() error {
	switch u.providerName {
	case "kiro":
		u.provider = kiroprovider.NewProvider()
	default:
		return fmt.Errorf("不支持的 provider: %q，支持的 provider 列表：%v", u.provider, "kiro")
	}
	info, err := os.Stat(u.credFile)
	if err != nil {
		return fmt.Errorf("访问路径失败: %w", err)
	}

	if info.IsDir() {
		return u.runDir(u.credFile)
	}
	return u.runFile(u.credFile)
}

// runDir 递归处理目录下所有 .json 文件
func (u *usageViewer) runDir(dir string) error {
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
		if err := u.runFile(path); err != nil {
			failureCount++
			return nil
		}
		successCount++
		return nil
	})

	log.Infof("查看完成！总凭证数量: %d, 成功：%d，失败：%d\n", successCount+failureCount, successCount, failureCount)
	return err
}

// runFile 读取并显示单个凭证文件的用量信息
func (u *usageViewer) runFile(filePath string) error {
	// 读取凭证文件
	creds, err := auth.LoadCredentials(u.providerName, filePath)
	if err != nil {
		return err
	}

	usageRules, err := u.provider.GetUsage(context.Background(), creds)
	if err != nil {
		return fmt.Errorf("获取用量信息失败: %w", err)
	}

	// 显示用量信息
	fmt.Printf("\n=== 凭证用量信息 (%s) ===\n", filePath)
	for _, rule := range usageRules {
		switch rule.TimeGranularity {
		case "day":
			fmt.Printf("日限额: %.0f / %.0f (剩余: %.0f)\n", rule.Used, rule.Total, rule.Remain)
		case "month":
			fmt.Printf("月限额: %.0f / %.0f (剩余: %.0f)\n", rule.Used, rule.Total, rule.Remain)
		default:
			fmt.Printf("%s限额: %.0f / %.0f (剩余: %.0f)\n", rule.TimeGranularity, rule.Used, rule.Total, rule.Remain)
		}
	}

	return nil
}
