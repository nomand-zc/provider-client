package cmd

import (
	"github.com/nomand-zc/provider-client/cli/cmd/scan"
	"github.com/nomand-zc/provider-client/cli/cmd/stream"
	"github.com/nomand-zc/provider-client/cli/cmd/token"
	"github.com/nomand-zc/provider-client/cli/cmd/usage"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "provider-client",
	Short: "provider-client 是一个 AI Provider 客户端工具",
	Long:  `provider-client 提供了与各 AI Provider 交互的命令行工具，包括凭证管理、token 刷新等功能。`,
}

// Execute 执行根命令
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(token.CMD())
	rootCmd.AddCommand(usage.CMD())
	rootCmd.AddCommand(stream.CMD())
	rootCmd.AddCommand(scan.CMD())
}
