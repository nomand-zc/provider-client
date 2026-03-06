package usage

import "github.com/spf13/cobra"

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