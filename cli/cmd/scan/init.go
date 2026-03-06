package scan

import "github.com/spf13/cobra"

// CMD 返回 scan 子命令
func CMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "扫描并校验凭证文件",
		Long:  `从指定目录下递归扫描所有 JSON 凭证文件，校验凭证是否有效，并将凭证文件按状态移动到对应目录。`,
	}

	// 注册子命令
	cmd.AddCommand(
		defaultCredScanner.cmd(),
	)

	return cmd
}
