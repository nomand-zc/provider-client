package token

import "github.com/spf13/cobra"

// CMD 返回 token 子命令，并注册所有 token 相关子命令
func CMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "token 凭证管理",
		Long:  `管理各 AI Provider 的 token 凭证，包括刷新等操作。`,
	}

	// 注册子命令
	cmd.AddCommand(
		defaultRefresher.cmd(),
	)

	return cmd
}
