package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	providerclient "github.com/nomand-zc/token101/provider-client"
	"github.com/nomand-zc/token101/provider-client/client/kiro"
	"github.com/nomand-zc/token101/provider-client/types"
	"github.com/nomand-zc/token101/provider-client/types/httperror"
)

// ============================================================
// 凭证配置
// ============================================================

// loadCredentials 从 JSON 文件中加载 Kiro 凭证
// credentialsFile: JSON 凭证文件路径，文件内容示例：
//
//	{
//	  "access_token": "your-access-token-here",
//	  "refresh_token": "your-refresh-token-here",
//	  "profile_arn": "arn:aws:codewhisperer:us-east-1:xxxxxxxxxxxx:profile/XXXXXXXXXXXXXXXX",
//	  "auth_method": "social",
//	  "provider": "Google",
//	  "region": "us-east-1"
//	}
func loadCredentials(credentialsFile string) *kiro.Credentials {
	if credentialsFile == "" {
		log.Fatal("❌ 请通过 -credentials 参数指定凭证文件路径，例如：-credentials /path/to/credentials.json")
	}

	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		log.Fatalf("❌ 读取凭证文件失败: %v", err)
	}

	var creds kiro.Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		log.Fatalf("❌ 解析凭证文件失败: %v", err)
	}

	return &creds
}

// ============================================================
// 场景一：基础非流式对话
// ============================================================

// example1BasicChat 演示最简单的单轮对话
func example1BasicChat(ctx context.Context, client providerclient.Model, creds *kiro.Credentials, model string) {
	fmt.Println("\n========== 场景一：基础非流式对话 ==========")

	req := types.Request{
		Model: model,
		Messages: []types.Message{
			types.NewUserMessage("你好，请用一句话介绍一下你自己。"),
		},
	}

	resp, err := client.GenerateContent(ctx, creds, req)
	if err != nil {
		log.Printf("❌ 请求失败: %v", err)
		return
	}

	if resp.Error != nil {
		log.Printf("❌ 响应错误: %s", resp.Error.Message)
		return
	}

	if len(resp.Choices) > 0 {
		fmt.Printf("✅ 回复: %s\n", resp.Choices[0].Message.Content)
	}

	// 打印 token 用量（如果有）
	if resp.Usage != nil {
		fmt.Printf("   Token 用量 - 输入: %d, 输出: %d, 总计: %d\n",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	}
}

// ============================================================
// 场景二：流式对话
// ============================================================

// example2StreamChat 演示流式输出，实时打印每个响应块
func example2StreamChat(ctx context.Context, client providerclient.Model, creds *kiro.Credentials, model string) {
	fmt.Println("\n========== 场景二：流式对话 ==========")

	req := types.Request{
		Model: model,
		Messages: []types.Message{
			types.NewUserMessage("请用 3 句话描述 Go 语言的主要特点。"),
		},
	}

	chain, err := client.GenerateContentStream(ctx, creds, req)
	if err != nil {
		log.Printf("❌ 流式请求失败: %v", err)
		return
	}

	fmt.Print("✅ 流式回复: ")
	var fullContent strings.Builder
	for resp := range chain {
		if resp == nil {
			continue
		}
		if resp.Error != nil {
			log.Printf("\n❌ 流式响应错误: %s", resp.Error.Message)
			return
		}
		// 打印每个内容块
		for _, choice := range resp.Choices {
			if choice.Delta.Content != "" {
				fmt.Print(choice.Delta.Content)
				fullContent.WriteString(choice.Delta.Content)
			}
		}
		// 流结束
		if resp.Done {
			fmt.Println()
			break
		}
	}

	fmt.Printf("   完整内容长度: %d 字符\n", len([]rune(fullContent.String())))
}

// ============================================================
// 场景三：带系统提示词的对话
// ============================================================

// example3WithSystemPrompt 演示如何使用系统提示词设定 AI 角色
func example3WithSystemPrompt(ctx context.Context, client providerclient.Model, creds *kiro.Credentials, model string) {
	fmt.Println("\n========== 场景三：带系统提示词的对话 ==========")

	req := types.Request{
		Model: model,
		Messages: []types.Message{
			// 系统提示词：设定 AI 角色
			{
				Role:    types.RoleSystem,
				Content: "你是一位专业的 Go 语言代码审查专家，回答简洁专业，每次回答不超过 100 字。",
			},
			types.NewUserMessage("如何避免 Go 中的内存泄漏？"),
		},
	}

	resp, err := client.GenerateContent(ctx, creds, req)
	if err != nil {
		log.Printf("❌ 请求失败: %v", err)
		return
	}

	if resp.Error != nil {
		log.Printf("❌ 响应错误: %s", resp.Error.Message)
		return
	}

	if len(resp.Choices) > 0 {
		fmt.Printf("✅ 专家回复: %s\n", resp.Choices[0].Message.Content)
	}
}

// ============================================================
// 场景四：多轮对话（携带历史消息）
// ============================================================

// example4MultiTurnChat 演示多轮对话，携带完整的对话历史
func example4MultiTurnChat(ctx context.Context, client providerclient.Model, creds *kiro.Credentials, model string) {
	fmt.Println("\n========== 场景四：多轮对话 ==========")

	req := types.Request{
		Model: model,
		Messages: []types.Message{
			// 第一轮
			types.NewUserMessage("我在学习 Go 语言，请推荐一个入门项目。"),
			types.NewAssistantMessage("推荐你做一个简单的 HTTP 服务器，可以学到路由、中间件、JSON 处理等核心概念。"),
			// 第二轮
			types.NewUserMessage("好主意！能给我一个最简单的 HTTP 服务器代码示例吗？"),
			types.NewAssistantMessage("当然！以下是最简单的 Go HTTP 服务器：\n```go\npackage main\n\nimport (\n    \"fmt\"\n    \"net/http\"\n)\n\nfunc main() {\n    http.HandleFunc(\"/\", func(w http.ResponseWriter, r *http.Request) {\n        fmt.Fprintln(w, \"Hello, World!\")\n    })\n    http.ListenAndServe(\":8080\", nil)\n}\n```"),
			// 第三轮（当前问题）
			types.NewUserMessage("如何给这个服务器添加一个 /health 健康检查接口？"),
		},
	}

	chain, err := client.GenerateContentStream(ctx, creds, req)
	if err != nil {
		log.Printf("❌ 流式请求失败: %v", err)
		return
	}

	fmt.Print("✅ 多轮对话回复: ")
	for resp := range chain {
		if resp == nil {
			continue
		}
		if resp.Error != nil {
			log.Printf("\n❌ 流式响应错误: %s", resp.Error.Message)
			return
		}
		for _, choice := range resp.Choices {
			if choice.Delta.Content != "" {
				fmt.Print(choice.Delta.Content)
			}
		}
		if resp.Done {
			fmt.Println()
			break
		}
	}
}

// ============================================================
// 场景五：工具调用（Function Calling）
// ============================================================

// weatherTool 天气查询工具
type weatherTool struct{}

func (t *weatherTool) Declaration() *types.Declaration {
	return &types.Declaration{
		Name:        "get_weather",
		Description: "获取指定城市的当前天气信息",
		InputSchema: &types.Schema{
			Type:     "object",
			Required: []string{"city"},
			Properties: map[string]*types.Schema{
				"city": {
					Type:        "string",
					Description: "城市名称，例如：北京、上海、广州",
				},
				"unit": {
					Type:        "string",
					Description: "温度单位，celsius（摄氏度）或 fahrenheit（华氏度）",
					Enum:        []any{"celsius", "fahrenheit"},
					Default:     "celsius",
				},
			},
		},
	}
}

// calculatorTool 计算器工具
type calculatorTool struct{}

func (t *calculatorTool) Declaration() *types.Declaration {
	return &types.Declaration{
		Name:        "calculate",
		Description: "执行数学计算，支持加减乘除",
		InputSchema: &types.Schema{
			Type:     "object",
			Required: []string{"expression"},
			Properties: map[string]*types.Schema{
				"expression": {
					Type:        "string",
					Description: "数学表达式，例如：2 + 3 * 4",
				},
			},
		},
	}
}

// example5ToolCalling 演示工具调用场景
func example5ToolCalling(ctx context.Context, client providerclient.Model, creds *kiro.Credentials, model string) {
	fmt.Println("\n========== 场景五：工具调用 ==========")

	req := types.Request{
		Model: model,
		Messages: []types.Message{
			types.NewUserMessage("请帮我查询北京今天的天气，并计算一下 15 * 8 + 32 的结果。"),
		},
		Tools: map[string]types.Tool{
			"get_weather": &weatherTool{},
			"calculate":   &calculatorTool{},
		},
	}

	resp, err := client.GenerateContent(ctx, creds, req)
	if err != nil {
		// Kiro API 对工具调用格式有特殊要求，400 是已知的预期行为
		if httpErr, ok := err.(*httperror.HTTPError); ok && httpErr.RawStatusCode == 400 {
			fmt.Printf("⚠️  Kiro API 工具调用返回 400（工具格式限制）: %v\n", err)
			return
		}
		log.Printf("❌ 请求失败: %v", err)
		return
	}

	if resp.Error != nil {
		log.Printf("❌ 响应错误: %s", resp.Error.Message)
		return
	}

	if len(resp.Choices) == 0 {
		fmt.Println("⚠️  无响应内容")
		return
	}

	choice := resp.Choices[0]

	// 检查是否有工具调用
	if len(choice.Message.ToolCalls) > 0 {
		fmt.Println("✅ AI 请求调用工具:")
		for _, tc := range choice.Message.ToolCalls {
			argsJSON, _ := json.MarshalIndent(tc.Function.Arguments, "   ", "  ")
			fmt.Printf("   工具: %s\n   参数: %s\n", tc.Function.Name, string(argsJSON))
		}

		// 模拟工具执行结果，构建第二轮请求
		fmt.Println("\n   模拟执行工具，构建第二轮请求...")
		messages := []types.Message{
			types.NewUserMessage("请帮我查询北京今天的天气，并计算一下 15 * 8 + 32 的结果。"),
			// 将 AI 的工具调用请求加入历史
			{
				Role:      types.RoleAssistant,
				ToolCalls: choice.Message.ToolCalls,
			},
		}

		// 添加工具执行结果
		for _, tc := range choice.Message.ToolCalls {
			var result string
			switch tc.Function.Name {
			case "get_weather":
				result = `{"city": "北京", "temperature": 22, "unit": "celsius", "condition": "晴天", "humidity": 45}`
			case "calculate":
				result = `{"result": 152, "expression": "15 * 8 + 32"}`
			default:
				result = `{"error": "unknown tool"}`
			}
			messages = append(messages, types.Message{
				Role:     types.RoleTool,
				ToolID:   tc.ID,
				ToolName: tc.Function.Name,
				Content:  result,
			})
		}

		// 发送第二轮请求（携带工具结果）
		req2 := types.Request{Model: model, Messages: messages}
		resp2, err := client.GenerateContent(ctx, creds, req2)
		if err != nil {
			log.Printf("❌ 第二轮请求失败: %v", err)
			return
		}
		if len(resp2.Choices) > 0 {
			fmt.Printf("✅ 最终回复: %s\n", resp2.Choices[0].Message.Content)
		}
	} else {
		// 直接返回文本（未触发工具调用）
		fmt.Printf("✅ 直接回复（未触发工具调用）: %s\n", choice.Message.Content)
	}
}

// ============================================================
// 场景六：JSON 格式凭证传入
// ============================================================

// example6JSONCredentials 演示使用 JSON 字节切片作为凭证
func example6JSONCredentials(ctx context.Context, client providerclient.Model, creds *kiro.Credentials, model string) {
	fmt.Println("\n========== 场景六：JSON 格式凭证 ==========")

	// 将凭证序列化为 JSON 字节切片（模拟从数据库或配置中心读取的场景）
	credsJSON, err := json.Marshal(creds)
	if err != nil {
		log.Printf("❌ 序列化凭证失败: %v", err)
		return
	}

	req := types.Request{
		Model: model,
		Messages: []types.Message{
			types.NewUserMessage("请说'JSON凭证测试成功'这几个字。"),
		},
	}

	// 直接传入 JSON 字节切片，客户端会自动解析
	resp, err := client.GenerateContent(ctx, credsJSON, req)
	if err != nil {
		log.Printf("❌ 请求失败: %v", err)
		return
	}

	if resp.Error != nil {
		log.Printf("❌ 响应错误: %s", resp.Error.Message)
		return
	}

	if len(resp.Choices) > 0 {
		fmt.Printf("✅ 回复: %s\n", resp.Choices[0].Message.Content)
	}
}

// ============================================================
// 场景七：自定义区域和选项
// ============================================================

// example7CustomOptions 演示自定义客户端选项
func example7CustomOptions(ctx context.Context, creds *kiro.Credentials, model string) {
	fmt.Println("\n========== 场景七：自定义客户端选项 ==========")

	// 创建自定义选项的客户端
	customClient := kiro.New(
		// 自定义默认区域（凭证中的 Region 优先级更高）
		kiro.WithDefaultRegion("us-west-2"),
		// 添加自定义请求头
		kiro.WithHeaders(map[string]string{
			"X-Custom-Header": "my-app/1.0",
		}),
	)

	req := types.Request{
		Model: model,
		Messages: []types.Message{
			types.NewUserMessage("请回答：2 + 2 = ？只需回答数字。"),
		},
	}

	resp, err := customClient.GenerateContent(ctx, creds, req)
	if err != nil {
		log.Printf("❌ 请求失败: %v", err)
		return
	}

	if resp.Error != nil {
		log.Printf("❌ 响应错误: %s", resp.Error.Message)
		return
	}

	if len(resp.Choices) > 0 {
		fmt.Printf("✅ 回复: %s\n", resp.Choices[0].Message.Content)
	}
}

// ============================================================
// 场景八：错误处理
// ============================================================

// example8ErrorHandling 演示各种错误场景的处理方式
func example8ErrorHandling(ctx context.Context, creds *kiro.Credentials) {
	fmt.Println("\n========== 场景八：错误处理 ==========")

	client := kiro.New()

	// 8.1 空消息列表错误
	fmt.Println("--- 8.1 空消息列表 ---")
	_, err := client.GenerateContent(ctx, creds, types.Request{
		Messages: []types.Message{},
	})
	if err != nil {
		fmt.Printf("✅ 捕获到预期错误: %v\n", err)
	}

	// 8.2 无效凭证类型错误
	fmt.Println("--- 8.2 无效凭证类型 ---")
	_, err = client.GenerateContent(ctx, 12345, types.Request{
		Messages: []types.Message{types.NewUserMessage("你好")},
	})
	if err != nil {
		fmt.Printf("✅ 捕获到预期错误: %v\n", err)
	}

	// 8.3 无效 AccessToken（HTTP 403 错误）
	fmt.Println("--- 8.3 无效 AccessToken（会发起真实 HTTP 请求）---")
	invalidCreds := &kiro.Credentials{
		AccessToken: "invalid_token_xxx",
		Region:      "us-east-1",
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err = client.GenerateContent(ctxWithTimeout, invalidCreds, types.Request{
		Messages: []types.Message{types.NewUserMessage("你好")},
	})
	if err != nil {
		// 判断错误类型
		if httpErr, ok := err.(*httperror.HTTPError); ok {
			fmt.Printf("✅ HTTP 错误 - 状态码: %d, 类型: %s\n",
				httpErr.RawStatusCode, httpErr.ErrorType)
			switch httpErr.ErrorType {
			case httperror.ErrorTypeForbidden:
				fmt.Println("   → 凭证无效或已过期，需要重新获取 AccessToken")
			case httperror.ErrorTypeRateLimit:
				fmt.Println("   → 请求频率超限，请稍后重试")
			case httperror.ErrorTypeServerError:
				fmt.Println("   → 服务端错误，请稍后重试")
			}
		} else {
			fmt.Printf("✅ 捕获到错误: %v\n", err)
		}
	}

	// 8.4 流式接口的错误处理（错误通过 channel 传递）
	fmt.Println("--- 8.4 流式接口错误处理 ---")
	chain, err := client.GenerateContentStream(ctxWithTimeout, invalidCreds, types.Request{
		Messages: []types.Message{types.NewUserMessage("你好")},
	})
	if err != nil {
		// 某些错误会直接返回（如凭证解析失败）
		fmt.Printf("✅ 流式接口直接返回错误: %v\n", err)
	} else {
		// 大多数错误通过 channel 传递
		for resp := range chain {
			if resp != nil && resp.Error != nil {
				fmt.Printf("✅ 流式接口 channel 错误: %s\n", resp.Error.Message)
				break
			}
		}
	}
}

// ============================================================
// 场景九：Context 超时与取消
// ============================================================

// example9ContextControl 演示 context 超时和取消的使用
func example9ContextControl(creds *kiro.Credentials, model string) {
	fmt.Println("\n========== 场景九：Context 超时与取消 ==========")

	client := kiro.New()

	// 9.1 设置超时
	fmt.Println("--- 9.1 带超时的请求 ---")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.GenerateContent(ctx, creds, types.Request{
		Model: model,
		Messages: []types.Message{
			types.NewUserMessage("请简单介绍一下 context 包在 Go 中的作用。"),
		},
	})
	if err != nil {
		log.Printf("❌ 请求失败: %v", err)
		return
	}
	if len(resp.Choices) > 0 {
		fmt.Printf("✅ 回复: %s\n", resp.Choices[0].Message.Content)
	}

	// 9.2 主动取消流式请求
	fmt.Println("--- 9.2 主动取消流式请求 ---")
	streamCtx, streamCancel := context.WithTimeout(context.Background(), 60*time.Second)

	chain, err := client.GenerateContentStream(streamCtx, creds, types.Request{
		Model: model,
		Messages: []types.Message{
			types.NewUserMessage("请写一篇关于 Go 语言并发编程的详细文章，至少 500 字。"),
		},
	})
	if err != nil {
		streamCancel()
		log.Printf("❌ 流式请求失败: %v", err)
		return
	}

	// 接收前几个响应块后主动取消
	count := 0
	for resp := range chain {
		if resp == nil {
			continue
		}
		count++
		for _, choice := range resp.Choices {
			if choice.Delta.Content != "" {
				fmt.Print(choice.Delta.Content)
			}
		}
		// 接收到 2 个响应块后取消
		if count >= 2 {
			fmt.Println("\n   [主动取消，停止接收后续内容]")
			streamCancel()
			break
		}
	}
	// 排空 channel，避免 goroutine 泄漏
	for range chain {
	}
	streamCancel()
	fmt.Printf("✅ 主动取消成功，共接收 %d 个响应块\n", count)
}

// ============================================================
// 主函数：运行所有示例
// ============================================================

func main() {
	// 命令行参数定义
	examplesFlag := flag.String("examples", "all", "要运行的示例编号，逗号分隔（如 \"1,2,3\"），或 \"all\" 运行全部")
	modelFlag := flag.String("model", "claude-sonnet-4-5", "使用的模型名称（如 claude-sonnet-4-5、claude-haiku-4-5、claude-opus-4-5）")
	credentialsFlag := flag.String("creds", "./creds.json", "JSON 凭证文件路径（必填），文件内容为 kiro.Credentials 的 JSON 格式")
	flag.Parse()

	fmt.Println("============================================================")
	fmt.Println("  Kiro (AWS CodeWhisperer) 渠道调用示例")
	fmt.Println("============================================================")
	fmt.Println()
	fmt.Printf("  模型: %s\n", *modelFlag)
	fmt.Printf("  示例: %s\n", *examplesFlag)
	fmt.Println()
	fmt.Println("⚠️  注意：请通过 -credentials 参数指定 JSON 凭证文件路径")
	fmt.Println("   凭证获取方式：从 Kiro IDE 的登录状态中提取 AccessToken，保存为 JSON 文件")
	fmt.Println()

	// 解析要运行的示例编号
	runAll := *examplesFlag == "all"
	selectedExamples := map[int]bool{}
	if !runAll {
		for _, s := range strings.Split(*examplesFlag, ",") {
			s = strings.TrimSpace(s)
			if n, err := strconv.Atoi(s); err == nil {
				selectedExamples[n] = true
			} else {
				fmt.Printf("⚠️  无效的示例编号: %q，已忽略\n", s)
			}
		}
	}

	shouldRun := func(n int) bool {
		return runAll || selectedExamples[n]
	}

	// 从 JSON 文件加载凭证
	creds := loadCredentials(*credentialsFlag)

	// 创建客户端
	client := kiro.New()

	// 设置全局超时 context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 运行各场景示例
	if shouldRun(1) {
		example1BasicChat(ctx, client, creds, *modelFlag)
	}
	if shouldRun(2) {
		example2StreamChat(ctx, client, creds, *modelFlag)
	}
	if shouldRun(3) {
		example3WithSystemPrompt(ctx, client, creds, *modelFlag)
	}
	if shouldRun(4) {
		example4MultiTurnChat(ctx, client, creds, *modelFlag)
	}
	if shouldRun(5) {
		example5ToolCalling(ctx, client, creds, *modelFlag)
	}
	if shouldRun(6) {
		example6JSONCredentials(ctx, client, creds, *modelFlag)
	}
	if shouldRun(7) {
		example7CustomOptions(ctx, creds, *modelFlag)
	}
	if shouldRun(8) {
		example8ErrorHandling(ctx, creds)
	}
	if shouldRun(9) {
		example9ContextControl(creds, *modelFlag)
	}

	fmt.Println("\n============================================================")
	fmt.Println("  所有示例执行完毕")
	fmt.Println("============================================================")
}
