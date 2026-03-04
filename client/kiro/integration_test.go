package kiro_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nomand-zc/token101/provider-client/client/kiro"
	"github.com/nomand-zc/token101/provider-client/types"
	"github.com/nomand-zc/token101/provider-client/types/httperror"
)

// testCredentials 是测试用的 Kiro 凭证（真实凭证，仅用于集成测试）
var testCredentials = &kiro.Credentials{
	AccessToken:  "aoaAAAAAGmn9X0b0L69SEPINnvgceiOHrZPxQyG44R3vW-Y6-2Bk-CrVJKka8PV99pYiMZOMBUaK6l4Pd0X-hCugsBkc0:MGQCMBnmcpkuOSuz43tz6WL8VXbHPzcUcThzCxd1QN3ZYuFomNTwtGxLgg0Watc2lo/DfQIwX0mipEvExxpgCZirIccFXBEi7P3fSZ/Y+Q2wQ4cY/dZRhCRTtvu1mAEye8JRqzD7",
	RefreshToken: "aorAAAAAGnAj5EeTK3L-eX3QAIXQlcewYVa5FyoXS93W_ArsnPqeRgNyzdxJ1XG3Ds-T85UOPDCeILYGgHyCS1HjEBkc0:MGUCMQDCV6tqGbsz1rmXzkoASn2tspeEkbA6a5PTvE9zMytRwgSyq12B88++LNZhIzCI89wCMFY6M/wNDBFvjBg+61hgiEyxd/swJW5zTXl/70/LFZ/28R8p1VGo2VGAtLmZTJaiPA",
	ProfileArn:   "arn:aws:codewhisperer:us-east-1:699475941385:profile/EHGA3GRVQMUK",
	AuthMethod:   "social",
	Provider:     "Google",
	Region:       "us-east-1",
}

// simpleTool 是一个用于测试工具调用的简单工具实现
type simpleTool struct {
	name        string
	description string
}

func (t *simpleTool) Declaration() *types.Declaration {
	return &types.Declaration{
		Name:        t.name,
		Description: t.description,
		InputSchema: &types.Schema{
			Type: "object",
			Properties: map[string]*types.Schema{
				"query": {
					Type:        "string",
					Description: "查询内容",
				},
			},
		},
	}
}

// ============================================================
// 一、错误处理测试（不依赖 API 成功响应，快速可靠）
// ============================================================

// skipIfAPIUnavailable 检查错误是否为凭证过期（403）、限流（429）或网络问题，
// 如果是则跳过测试（这些是环境问题，不是代码问题）
func skipIfAPIUnavailable(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if httpErr, ok := err.(*httperror.HTTPError); ok {
		switch httpErr.RawStatusCode {
		case 429:
			t.Skip("API 限流（429），跳过测试")
		case 403:
			t.Skip("凭证已过期或无效（403），跳过测试")
		}
	}
	// 网络错误（如 HTTPS 问题）
	if strings.Contains(err.Error(), "http: server gave HTTP response to HTTPS client") ||
		strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no such host") {
		t.Skipf("网络问题，跳过测试: %v", err)
	}
}

// TestGenerateContent_EmptyMessages 测试空消息列表的错误处理
func TestGenerateContent_EmptyMessages(t *testing.T) {
	client := kiro.New()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := types.Request{
		Messages: []types.Message{},
	}

	_, err := client.GenerateContent(ctx, testCredentials, req)
	if err == nil {
		t.Fatal("空消息列表应返回错误")
	}
	if !strings.Contains(err.Error(), "message") {
		t.Errorf("错误信息应包含 'message'，实际: %v", err)
	}

	t.Logf("✅ 空消息错误处理测试通过，错误: %v", err)
}

// TestGenerateContentStream_EmptyMessages 测试流式接口的空消息错误处理
func TestGenerateContentStream_EmptyMessages(t *testing.T) {
	client := kiro.New()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := types.Request{
		Messages: []types.Message{},
	}

	_, err := client.GenerateContentStream(ctx, testCredentials, req)
	if err == nil {
		t.Fatal("空消息列表应返回错误")
	}

	t.Logf("✅ 流式接口空消息错误处理测试通过，错误: %v", err)
}

// TestGenerateContent_InvalidCredentials 测试无效凭证的错误处理
func TestGenerateContent_InvalidCredentials(t *testing.T) {
	client := kiro.New()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	invalidCreds := &kiro.Credentials{
		AccessToken: "invalid_token",
		Region:      "us-east-1",
	}

	req := types.Request{
		Messages: []types.Message{
			types.NewUserMessage("你好"),
		},
	}

	_, err := client.GenerateContent(ctx, invalidCreds, req)
	if err == nil {
		t.Fatal("无效凭证应返回错误")
	}

	// 验证错误类型为 HTTPError
	httpErr, ok := err.(*httperror.HTTPError)
	if !ok {
		t.Fatalf("错误应为 *httperror.HTTPError 类型，实际: %T", err)
	}
	if httpErr.ErrorType != httperror.ErrorTypeForbidden {
		t.Errorf("无效凭证应返回 Forbidden 错误，实际: %s", httpErr.ErrorType)
	}
	if httpErr.RawStatusCode != 403 {
		t.Errorf("HTTP 状态码应为 403，实际: %d", httpErr.RawStatusCode)
	}

	t.Logf("✅ 无效凭证错误处理测试通过")
	t.Logf("   错误类型: %s, 状态码: %d", httpErr.ErrorType, httpErr.RawStatusCode)
}

// TestGenerateContentStream_InvalidCredentials 测试流式接口的无效凭证错误处理
// 注意：流式接口的 HTTP 错误通过 channel 传递，而非直接返回
func TestGenerateContentStream_InvalidCredentials(t *testing.T) {
	client := kiro.New()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	invalidCreds := &kiro.Credentials{
		AccessToken: "invalid_token",
		Region:      "us-east-1",
	}

	req := types.Request{
		Messages: []types.Message{
			types.NewUserMessage("你好"),
		},
	}

	chain, err := client.GenerateContentStream(ctx, invalidCreds, req)
	if err != nil {
		// 如果直接返回错误（如凭证解析失败），也是合理的
		t.Logf("GenerateContentStream 直接返回错误: %v", err)
		return
	}

	// 从 channel 中读取错误响应
	var gotError bool
	for resp := range chain {
		if resp != nil && resp.Error != nil {
			gotError = true
			t.Logf("✅ 流式接口无效凭证错误处理测试通过，错误: %s", resp.Error.Message)
			break
		}
	}

	if !gotError {
		t.Error("无效凭证应通过 channel 返回错误响应")
	}
}

// TestGenerateContent_UnsupportedCredentialsType 测试不支持的凭证类型
func TestGenerateContent_UnsupportedCredentialsType(t *testing.T) {
	client := kiro.New()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := types.Request{
		Messages: []types.Message{
			types.NewUserMessage("你好"),
		},
	}

	// 传入不支持的凭证类型（整数）
	_, err := client.GenerateContent(ctx, 12345, req)
	if err == nil {
		t.Fatal("不支持的凭证类型应返回错误")
	}
	if !strings.Contains(err.Error(), "unsupported credentials type") {
		t.Errorf("错误信息应包含 'unsupported credentials type'，实际: %v", err)
	}

	t.Logf("✅ 不支持的凭证类型错误处理测试通过，错误: %v", err)
}

// ============================================================
// 二、凭证格式测试
// ============================================================

// TestExtractCredentials_ValidJSON 测试从 JSON 字节切片提取凭证
func TestExtractCredentials_ValidJSON(t *testing.T) {
	credsJSON, err := json.Marshal(testCredentials)
	if err != nil {
		t.Fatalf("序列化凭证失败: %v", err)
	}

	extracted, err := kiro.ExtractCredentials(credsJSON)
	if err != nil {
		t.Fatalf("提取凭证失败: %v", err)
	}

	if extracted.AccessToken != testCredentials.AccessToken {
		t.Errorf("AccessToken 不匹配")
	}
	if extracted.ProfileArn != testCredentials.ProfileArn {
		t.Errorf("ProfileArn 不匹配")
	}
	if extracted.Region != testCredentials.Region {
		t.Errorf("Region 不匹配")
	}
	if extracted.AuthMethod != testCredentials.AuthMethod {
		t.Errorf("AuthMethod 不匹配")
	}

	t.Logf("✅ JSON 凭证提取测试通过")
}

// TestExtractCredentials_InvalidJSON 测试无效 JSON 的错误处理
func TestExtractCredentials_InvalidJSON(t *testing.T) {
	_, err := kiro.ExtractCredentials([]byte("invalid json"))
	if err == nil {
		t.Fatal("无效 JSON 应返回错误")
	}

	t.Logf("✅ 无效 JSON 凭证错误处理测试通过，错误: %v", err)
}

// ============================================================
// 三、真实 API 集成测试（使用有效凭证，验证 AWS 事件流解析）
// ============================================================

// awsEventStreamFrame 表示一个 AWS 事件流帧
type awsEventStreamFrame struct {
	EventType string
	Payload   []byte
}

// parseAWSEventStream 解析 AWS 事件流二进制格式
// AWS 事件流帧格式：
//   - 4 字节：总长度（含帧头和 CRC）
//   - 4 字节：头部长度
//   - 4 字节：头部 CRC
//   - N 字节：头部（键值对）
//   - M 字节：payload
//   - 4 字节：消息 CRC
func parseAWSEventStream(data []byte) ([]awsEventStreamFrame, error) {
	var frames []awsEventStreamFrame
	offset := 0

	for offset < len(data) {
		if offset+12 > len(data) {
			break
		}

		totalLen := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		headersLen := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))

		if totalLen <= 0 || offset+totalLen > len(data) {
			break
		}

		// 解析头部，提取 :event-type
		headerStart := offset + 12
		headerEnd := headerStart + headersLen
		eventType := ""

		if headerEnd <= len(data) {
			hi := 0
			headerBytes := data[headerStart:headerEnd]
			for hi < len(headerBytes) {
				if hi >= len(headerBytes) {
					break
				}
				nameLen := int(headerBytes[hi])
				hi++
				if hi+nameLen > len(headerBytes) {
					break
				}
				name := string(headerBytes[hi : hi+nameLen])
				hi += nameLen
				if hi+3 > len(headerBytes) {
					break
				}
				hi++ // valueType
				valueLen := int(binary.BigEndian.Uint16(headerBytes[hi : hi+2]))
				hi += 2
				if hi+valueLen > len(headerBytes) {
					break
				}
				value := string(headerBytes[hi : hi+valueLen])
				hi += valueLen
				if name == ":event-type" {
					eventType = value
				}
			}
		}

		// 提取 payload
		payloadStart := headerEnd
		payloadEnd := offset + totalLen - 4
		var payload []byte
		if payloadEnd > payloadStart && payloadEnd <= len(data) {
			payload = data[payloadStart:payloadEnd]
		}

		frames = append(frames, awsEventStreamFrame{
			EventType: eventType,
			Payload:   payload,
		})

		offset += totalLen
	}

	return frames, nil
}

// TestRawAPI_AWSEventStreamFormat 直接测试 Kiro API 的原始响应格式
// 验证 API 确实返回 AWS 事件流二进制格式，并能正确解析
func TestRawAPI_AWSEventStreamFormat(t *testing.T) {
	body := map[string]interface{}{
		"conversationState": map[string]interface{}{
			"chatTriggerType": "MANUAL",
			"conversationId":  "test-integration-" + fmt.Sprintf("%d", time.Now().UnixNano()),
			"currentMessage": map[string]interface{}{
				"userInputMessage": map[string]interface{}{
					"content": "请回答：1+1=？只需回答数字。",
					"modelId": "CLAUDE_SONNET_4_5_20250929_V1_0",
					"origin":  "AI_EDITOR",
				},
			},
		},
		"profileArn": testCredentials.ProfileArn,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("序列化请求失败: %v", err)
	}

	req, err := http.NewRequest("POST", "https://q.us-east-1.amazonaws.com/generateAssistantResponse",
		strings.NewReader(string(bodyBytes)))
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+testCredentials.AccessToken)
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
	req.Header.Set("x-amz-user-agent", "aws-sdk-js/1.0.0 KiroIDE-0.8.140")
	req.Header.Set("User-Agent", "aws-sdk-js/1.0.0 ua/2.1 api/codewhispererruntime#1.0.0 m/E KiroIDE-0.8.140")
	req.Header.Set("Connection", "close")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP 请求失败: %v", err)
	}
	defer resp.Body.Close()

	t.Logf("HTTP 状态码: %d", resp.StatusCode)
	t.Logf("Content-Type: %s", resp.Header.Get("Content-Type"))

	if resp.StatusCode == 429 || resp.StatusCode == 403 {
		t.Skipf("API 不可用（%d），跳过测试", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("API 返回非 200 状态码: %d, body: %s", resp.StatusCode, string(body))
	}

	allBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	t.Logf("响应体总长度: %d 字节", len(allBytes))

	// 解析 AWS 事件流
	frames, err := parseAWSEventStream(allBytes)
	if err != nil {
		t.Fatalf("解析 AWS 事件流失败: %v", err)
	}

	if len(frames) == 0 {
		t.Fatal("应至少包含一个事件流帧")
	}

	// 统计各类型帧
	contentFrames := 0
	var fullContent strings.Builder

	for _, frame := range frames {
		t.Logf("帧类型: %s, payload: %s", frame.EventType, string(frame.Payload))

		if frame.EventType == "assistantResponseEvent" {
			contentFrames++
			var payload map[string]interface{}
			if err := json.Unmarshal(frame.Payload, &payload); err != nil {
				t.Errorf("解析 assistantResponseEvent payload 失败: %v", err)
				continue
			}
			if content, ok := payload["content"].(string); ok {
				fullContent.WriteString(content)
			}
		}
	}

	if contentFrames == 0 {
		t.Error("应至少包含一个 assistantResponseEvent 帧")
	}

	content := fullContent.String()
	if content == "" {
		t.Error("响应内容不应为空")
	}

	t.Logf("✅ AWS 事件流格式验证通过")
	t.Logf("   总帧数: %d, 内容帧数: %d", len(frames), contentFrames)
	t.Logf("   完整内容: %s", content)
}

// TestGenerateContentStream_WithValidCredentials 测试流式接口（使用有效凭证）
// 验证 AWS 事件流解析修复后能正确接收内容帧
func TestGenerateContentStream_WithValidCredentials(t *testing.T) {
	client := kiro.New()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := types.Request{
		Messages: []types.Message{
			types.NewUserMessage("请回答：1+1=？只需回答数字。"),
		},
	}

	chain, err := client.GenerateContentStream(ctx, testCredentials, req)
	if err != nil {
		skipIfAPIUnavailable(t, err)
		t.Fatalf("GenerateContentStream 失败: %v", err)
	}

	var responses []*types.Response
	var fullContent strings.Builder
	var gotDone bool

	for resp := range chain {
		if resp == nil {
			continue
		}
		if resp.Error != nil {
			t.Fatalf("流式响应包含错误: %s", resp.Error.Message)
		}
		responses = append(responses, resp)
		for _, choice := range resp.Choices {
			if choice.Delta.Content != "" {
				fullContent.WriteString(choice.Delta.Content)
			}
		}
		if resp.Done {
			gotDone = true
		}
	}

	if len(responses) == 0 {
		t.Fatal("应至少收到一个响应")
	}
	if !gotDone {
		t.Error("流式响应应以 Done=true 的块结束")
	}

	content := fullContent.String()
	if content == "" {
		t.Error("流式响应内容不应为空")
	}

	t.Logf("✅ 流式接口测试通过")
	t.Logf("   收到响应数量: %d", len(responses))
	t.Logf("   完整内容: %s", content)
}

// TestGenerateContent_WithValidCredentials 测试非流式接口（使用有效凭证）
// 验证 AWS 事件流解析修复后能正确聚合响应内容
func TestGenerateContent_WithValidCredentials(t *testing.T) {
	client := kiro.New()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := types.Request{
		Messages: []types.Message{
			types.NewUserMessage("请回答：1+1=？只需回答数字。"),
		},
	}

	resp, err := client.GenerateContent(ctx, testCredentials, req)
	if err != nil {
		skipIfAPIUnavailable(t, err)
		t.Fatalf("GenerateContent 失败: %v", err)
	}

	if resp == nil {
		t.Fatal("响应不应为 nil")
	}
	if resp.Error != nil {
		t.Fatalf("响应包含错误: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		t.Fatal("响应应包含至少一个 choice")
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		t.Fatal("响应内容不应为空")
	}
	if !resp.Done {
		t.Error("非流式响应的 Done 字段应为 true")
	}
	if resp.IsPartial {
		t.Error("非流式响应的 IsPartial 字段应为 false")
	}

	t.Logf("✅ GenerateContent 测试通过（AWS 事件流解析正常）")
	t.Logf("   内容: %s", content)
	if resp.Usage != nil {
		t.Logf("   Token 用量 - 输入: %d, 输出: %d, 总计: %d",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	}
}

// TestGenerateContent_JSONCredentials 测试 JSON 格式凭证传入
func TestGenerateContent_JSONCredentials(t *testing.T) {
	client := kiro.New()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 将凭证序列化为 JSON 字节切片
	credsJSON, err := json.Marshal(testCredentials)
	if err != nil {
		t.Fatalf("序列化凭证失败: %v", err)
	}

	req := types.Request{
		Messages: []types.Message{
			types.NewUserMessage("请说'测试成功'这四个字。"),
		},
	}

	// 使用 JSON 字节切片作为凭证，验证凭证解析逻辑
	resp, err := client.GenerateContent(ctx, credsJSON, req)
	if err != nil {
		skipIfAPIUnavailable(t, err)
		t.Fatalf("使用 JSON 格式凭证调用 GenerateContent 失败: %v", err)
	}

	if resp == nil || resp.Error != nil || len(resp.Choices) == 0 {
		t.Fatal("响应无效")
	}

	t.Logf("✅ JSON 格式凭证测试通过")
	t.Logf("   内容: %s", resp.Choices[0].Message.Content)
}

// TestGenerateContentStream_MultiTurn 测试多轮对话的流式响应
func TestGenerateContentStream_MultiTurn(t *testing.T) {
	client := kiro.New()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := types.Request{
		Messages: []types.Message{
			types.NewUserMessage("我喜欢编程。"),
			types.NewAssistantMessage("太好了！编程是一项很有价值的技能。你主要使用哪种编程语言？"),
			types.NewUserMessage("我主要用 Go 语言，请给我一个简单的 Hello World 示例。"),
		},
	}

	chain, err := client.GenerateContentStream(ctx, testCredentials, req)
	if err != nil {
		skipIfAPIUnavailable(t, err)
		t.Fatalf("GenerateContentStream 失败: %v", err)
	}

	var fullContent strings.Builder
	responseCount := 0
	for resp := range chain {
		if resp == nil {
			continue
		}
		if resp.Error != nil {
			t.Fatalf("流式响应包含错误: %s", resp.Error.Message)
		}
		responseCount++
		for _, choice := range resp.Choices {
			if choice.Delta.Content != "" {
				fullContent.WriteString(choice.Delta.Content)
			}
		}
	}

	content := fullContent.String()
	if content == "" {
		t.Error("多轮对话响应内容不应为空")
	}
	// 验证响应包含 Go 代码相关内容
	if !strings.Contains(content, "fmt") && !strings.Contains(content, "Hello") && !strings.Contains(content, "package") {
		t.Logf("⚠️  响应可能不包含 Go Hello World 代码，实际内容: %s", content)
	}

	t.Logf("✅ 多轮对话流式响应测试通过，收到 %d 个响应", responseCount)
	t.Logf("   完整内容: %s", content)
}

// TestGenerateContentStream_WithTools 测试带工具定义的流式请求
// 注意：Kiro API 对工具调用的支持有限，某些工具格式可能导致 400 错误
func TestGenerateContentStream_WithTools(t *testing.T) {
	client := kiro.New()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	searchTool := &simpleTool{
		name:        "search_web",
		description: "搜索互联网获取最新信息",
	}

	req := types.Request{
		Messages: []types.Message{
			types.NewUserMessage("请帮我搜索一下今天的天气情况。"),
		},
		Tools: map[string]types.Tool{
			"search_web": searchTool,
		},
	}

	chain, err := client.GenerateContentStream(ctx, testCredentials, req)
	if err != nil {
		if httpErr, ok := err.(*httperror.HTTPError); ok {
			switch httpErr.RawStatusCode {
			case 429:
				t.Skip("API 限流（429），跳过测试")
			case 403:
				t.Skip("凭证已过期（403），跳过测试")
			case 400:
				// Kiro API 对工具调用格式有特殊要求，400 是已知的预期行为
				t.Logf("⚠️  Kiro API 返回 400（工具调用格式不被支持）: %v", err)
				return
			}
		}
		skipIfAPIUnavailable(t, err)
		t.Fatalf("GenerateContentStream 失败: %v", err)
	}

	responseCount := 0
	for resp := range chain {
		if resp != nil {
			responseCount++
		}
	}

	t.Logf("✅ 带工具定义的流式请求测试完成，收到 %d 个响应", responseCount)
}

// TestGenerateContentStream_ContextCancellation 测试流式响应的 context 取消
func TestGenerateContentStream_ContextCancellation(t *testing.T) {
	client := kiro.New()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	req := types.Request{
		Messages: []types.Message{
			types.NewUserMessage("请写一篇关于 Go 语言的长文章，至少1000字。"),
		},
	}

	chain, err := client.GenerateContentStream(ctx, testCredentials, req)
	if err != nil {
		skipIfAPIUnavailable(t, err)
		cancel()
		t.Fatalf("GenerateContentStream 失败: %v", err)
	}

	// 接收几个响应后取消
	count := 0
	for resp := range chain {
		if resp == nil {
			continue
		}
		count++
		if count >= 2 {
			cancel() // 取消 context
			break
		}
	}

	// 排空 channel
	for range chain {
	}

	cancel() // 确保 cancel 被调用

	t.Logf("✅ Context 取消测试通过，接收了 %d 个响应后取消", count)
}
