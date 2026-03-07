package usage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nomand-zc/provider-client/cli/internal/auth"
	"github.com/nomand-zc/provider-client/cli/internal/factory"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
)

var defaultModelsViewer modelsViewer

// modelsViewer 查看模型列表
// 不传 --file 时查看 provider 默认支持的模型列表
// 传入 --file 时查看指定凭证支持的模型列表
type modelsViewer struct {
	credFile     string
	providerName string
	provider     providers.Provider
}

// CMD 返回 usage 子命令，并注册所有 usage 相关子命令
func CMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "查看凭证用量信息",
		Long:  `查看各 AI Provider 凭证的用量信息，包括日限额、月限额、已使用量等。`,
	}

	// 注册子命令
	cmd.AddCommand(
		defaultUsageViewer.cmd(),
		defaultModelsViewer.cmd(),
	)

	return cmd
}

func (m *modelsViewer) cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "查看模型列表",
		Long: `查看模型列表。

不传 --file 时，查看 provider 默认支持的模型列表（无需凭证）。
传入 --file 时，查看指定凭证文件所支持的模型列表，支持单个文件或目录（递归处理目录下所有 .json 文件）。

支持的 provider：
  - kiro

示例：
  # 查看 provider 默认模型列表
  provider-client usage models --provider kiro

  # 查看凭证支持的模型列表
  provider-client usage models --file /path/to/credentials.json --provider kiro

  # 查看目录下所有凭证支持的模型列表
  provider-client usage models --file /path/to/creds-dir/ --provider kiro`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return m.run()
		},
	}

	cmd.Flags().StringVarP(&m.credFile, "file", "f", "", "凭证 JSON 文件路径或目录（可选，不传则查看 provider 默认模型列表）")
	cmd.Flags().StringVarP(&m.providerName, "provider", "p", "kiro", fmt.Sprintf("provider 名称，支持：%v", factory.SupportedProviders))

	return cmd
}

func (m *modelsViewer) run() error {
	var err error
	m.provider, err = factory.NewProvider(m.providerName)
	if err != nil {
		return err
	}

	// 未传 --file，查看 provider 默认模型列表
	if m.credFile == "" {
		return m.runDefault()
	}

	// 传入 --file，查看凭证支持的模型列表
	info, err := os.Stat(m.credFile)
	if err != nil {
		return fmt.Errorf("访问路径失败: %w", err)
	}

	if info.IsDir() {
		return m.runDir(m.credFile)
	}
	return m.runFile(m.credFile)
}

// runDefault 查看 provider 默认支持的模型列表（无需凭证）
func (m *modelsViewer) runDefault() error {
	models, err := m.provider.Models(context.Background())
	if err != nil {
		return fmt.Errorf("获取模型列表失败: %w", err)
	}

	log.Infof("\n===== %s 支持的模型列表 =====\n", m.providerName)
	for i, model := range models {
		log.Infof("  %d. %s\n", i+1, model)
	}
	log.Infof("\n共 %d 个模型\n", len(models))

	return nil
}

// runDir 递归处理目录下所有 .json 文件
func (m *modelsViewer) runDir(dir string) error {
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
		if err := m.runFile(path); err != nil {
			log.Warnf("\n处理凭证文件 %q 失败: %v", path, err)
			failureCount++
			return nil
		}
		successCount++
		return nil
	})

	log.Infof("\n查询完成！总凭证数量: %d, 成功：%d，失败：%d\n", successCount+failureCount, successCount, failureCount)
	return err
}

// runFile 读取凭证文件并显示其支持的模型列表
func (m *modelsViewer) runFile(filePath string) error {
	creds, err := auth.LoadCredentials(m.providerName, filePath)
	if err != nil {
		return err
	}

	models, err := m.provider.ListModels(context.Background(), creds)
	if err != nil {
		return fmt.Errorf("获取模型列表失败: %w", err)
	}

	log.Infof("\n===== 凭证支持的模型列表 (%s) =====\n", filePath)
	for i, model := range models {
		log.Infof("  %d. %s\n", i+1, model)
	}
	log.Infof("\n共 %d 个模型\n", len(models))

	return nil
}
