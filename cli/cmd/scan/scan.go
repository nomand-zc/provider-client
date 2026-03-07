package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nomand-zc/provider-client/cli/internal/auth"
	"github.com/nomand-zc/provider-client/cli/utils"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
	kiroprovider "github.com/nomand-zc/provider-client/providers/kiro"
)

var defaultCredScanner credScanner

// credScanner 持有 scan 命令的参数
type credScanner struct {
	srcDir       string
	destDir      string
	providerName string
	provider     providers.Provider
}

func (s *credScanner) cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "校验凭证文件并按状态分类移动",
		Long: `从指定目录下递归扫描所有 JSON 凭证文件，校验凭证是否有效，
并根据校验结果将凭证文件移动到目标目录下的对应子目录：
  - enable:  凭证有效且未触发限流
  - limit:   凭证有效但触发了限流（临时不可用）
  - disable: 凭证永久失效（GetUsage 返回错误）

校验规则：
  1. 通过 Validate() 方法校验凭证文件格式是否正确
  2. 通过 GetUsage() 获取凭证使用情况，返回 error 则视为永久失效
  3. 如果 usage 触发了限流，则视为临时不可用
  4. 如果 usage 未触发限流，则视为有效凭证

示例：
  provider-client scan check --src /path/from/creds --dest /path/to/output --provider kiro`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return s.run()
		},
	}

	cmd.Flags().StringVarP(&s.srcDir, "src", "s", "", "凭证文件所在的源目录（必填）")
	cmd.Flags().StringVarP(&s.destDir, "dest", "d", "", "分类后凭证文件的目标目录（必填）")
	cmd.Flags().StringVarP(&s.providerName, "provider", "p", "kiro", fmt.Sprintf("provider 名称，支持：%v", "kiro"))
	_ = cmd.MarkFlagRequired("src")
	_ = cmd.MarkFlagRequired("dest")

	return cmd
}

// credStatus 凭证状态
type credStatus string

const (
	statusEnable  credStatus = "enable"
	statusLimit   credStatus = "limit"
	statusDisable credStatus = "disable"
)

// run 执行扫描校验逻辑
func (s *credScanner) run() error {
	// 初始化 provider
	switch s.providerName {
	case "kiro":
		s.provider = kiroprovider.NewProvider()
	default:
		return fmt.Errorf("不支持的 provider: %q，支持的 provider 列表：%v", s.providerName, "kiro")
	}

	// 创建目标子目录
	if err := ensureDirs(s.destDir); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 统计计数
	var enableCount, limitCount, disableCount, invalidCount int

	// 递归扫描所有 JSON 文件
	err := filepath.Walk(s.srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() ||
			!strings.HasSuffix(strings.ToLower(info.Name()), ".json") {
			return nil
		}

		status, err := s.checkCredential(path)
		if err != nil {
			log.Warnf("\n凭证文件 %q 格式校验失败: %v，跳过", path, err)
			invalidCount++
			return nil
		}

		// 根据状态移动文件
		destPath := filepath.Join(s.destDir, string(status), info.Name())
		if err := utils.CopyFile(path, destPath); err != nil {
			log.Warnf("\n拷贝凭证文件 %q 到 %q 失败: %v", path, destPath, err)
			return nil
		}

		switch status {
		case statusEnable:
			enableCount++
			log.Infof("\n[有效] %s -> %s", path, destPath)
		case statusLimit:
			limitCount++
			log.Infof("\n[限流] %s -> %s", path, destPath)
		case statusDisable:
			disableCount++
			log.Infof("\n[失效] %s -> %s", path, destPath)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("扫描目录失败: %w", err)
	}

	total := enableCount + limitCount + disableCount + invalidCount
	log.Infof("\n扫描完成！总凭证数量: %d, 有效: %d, 限流: %d, 失效: %d, 格式无效: %d",
		total, enableCount, limitCount, disableCount, invalidCount)

	return nil
}

// checkCredential 校验单个凭证文件，返回凭证状态
func (s *credScanner) checkCredential(filePath string) (credStatus, error) {
	_, err := auth.GetCredentialsFromFile(s.provider, filePath)
	if err != nil {
		if providers.IsRateLimitError(err) {
			return statusLimit, nil
		}
		return statusDisable, nil
	}
	return statusEnable, nil
}

// ensureDirs 确保目标目录及其子目录存在，如果子目录已存在且有文件则先清空
func ensureDirs(destDir string) error {
	dirs := []string{
		filepath.Join(destDir, string(statusEnable)),
		filepath.Join(destDir, string(statusLimit)),
		filepath.Join(destDir, string(statusDisable)),
	}
	for _, dir := range dirs {
		// 如果子目录已存在，先删除（连同其中的文件一起清除）
		if _, err := os.Stat(dir); err == nil {
			log.Infof("\n子目录 %q 已存在，清空旧文件...", dir)
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("删除目录 %q 失败: %w", dir, err)
			}
		}
		// 重新创建空目录
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %q 失败: %w", dir, err)
		}
	}
	return nil
}
