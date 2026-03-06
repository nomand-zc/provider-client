package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nomand-zc/provider-client/cli/internal/auth"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
	kiroprovider "github.com/nomand-zc/provider-client/providers/kiro"
	"github.com/nomand-zc/provider-client/queue"
)

// streamer 持有 stream 命令的参数
type streamer struct {
	credFile     string
	dataFile     string
	providerName string
	provider     providers.Provider
}

var (
	defaultStreamer streamer
)

// CMD 返回 stream 子命令
func CMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "流式生成内容",
		Long: `通过流式方式生成 AI 内容，支持自动刷新过期 token。

支持的 provider：
  - kiro

示例：
  provider-client stream --creds /path/to/credentials.json --data /path/to/data.json --provider kiro
  provider-client stream --creds /path/to/credentials/directory --data /path/to/data.json --provider kiro`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return defaultStreamer.run()
		},
	}

	cmd.Flags().StringVarP(&defaultStreamer.credFile, "creds", "c", "", "凭证 JSON 文件路径或目录路径（必填）")
	cmd.Flags().StringVarP(&defaultStreamer.dataFile, "data", "d", "", "请求 JSON 文件路径（必填）")
	cmd.Flags().StringVarP(&defaultStreamer.providerName, "provider", "p", "kiro", fmt.Sprintf("provider 名称，支持：%v（必填）", "kiro"))

	_ = cmd.MarkFlagRequired("creds")
	_ = cmd.MarkFlagRequired("data")

	return cmd
}

// run 执行流式生成逻辑
func (s *streamer) run() error {
	// 初始化 provider
	switch s.providerName {
	case "kiro":
		s.provider = kiroprovider.NewProvider()
	default:
		return fmt.Errorf("不支持的 provider: %q，支持的 provider 列表：%v", s.providerName, "kiro")
	}

	creds, err := auth.GetValidCredentials(s.provider, s.credFile)
	if err != nil {
		return fmt.Errorf("凭证不可用: %w", err)
	}

	// 读取请求文件
	reqData, err := os.ReadFile(s.dataFile)
	if err != nil {
		return fmt.Errorf("读取请求文件失败: %w", err)
	}

	// 解析请求
	var req providers.Request
	if err := json.Unmarshal(reqData, &req); err != nil {
		return fmt.Errorf("解析请求 JSON 失败: %w", err)
	}
	// 确保启用流式模式
	req.GenerationConfig.Stream = true

	// 执行流式生成
	log.Infof("开始流式生成内容...")
	reader, err := s.provider.GenerateContentStream(context.Background(), creds, req)
	if err != nil {
		return fmt.Errorf("流式生成失败: %w", err)
	}

	// 处理流式响应
	for {
		response, err := reader.Read()
		if err != nil {
			if err == queue.ErrQueueClosed {
				break
			}
			return fmt.Errorf("读取流式响应失败: %w", err)
		}

		if response == nil {
			continue
		}

		// 输出内容
		rsp, _ := json.Marshal(response)
		fmt.Println(string(rsp))
	}

	log.Infof("\n流式生成完成")
	return nil
}
