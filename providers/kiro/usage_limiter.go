package kiro

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/juju/errors"
	"github.com/nomand-zc/provider-client/credentials"
	kirocreds "github.com/nomand-zc/provider-client/credentials/kiro"
	"github.com/nomand-zc/provider-client/limitrule"
	"github.com/nomand-zc/provider-client/providers"
)

const (
	usageURL = "https://q.%s.amazonaws.com/getUsageLimits?isEmailRequired=true&origin=AI_EDITOR&resourceType=AGENTIC_REQUEST"
)

// kiroUsageResp 表示 getUsageLimits 接口的顶层响应
type kiroUsageResp struct {
	DaysUntilReset     int                  `json:"daysUntilReset"`
	NextDateReset      float64              `json:"nextDateReset"`
	UsageBreakdownList []usageBreakdownItem `json:"usageBreakdownList"`
	SubscriptionInfo   *subscriptionInfo    `json:"subscriptionInfo"`
	UserInfo           *userInfo            `json:"userInfo"`
}

// usageBreakdownItem 表示 usageBreakdownList 中的单个条目
type usageBreakdownItem struct {
	UsageLimit                int            `json:"usageLimit"`
	CurrentUsage              int            `json:"currentUsage"`
	CurrentUsageWithPrecision float64        `json:"currentUsageWithPrecision"`
	UsageLimitWithPrecision   float64        `json:"usageLimitWithPrecision"`
	ResourceType              string         `json:"resourceType"`
	Unit                      string         `json:"unit"`
	DisplayName               string         `json:"displayName"`
	DisplayNamePlural         string         `json:"displayNamePlural"`
	FreeTrialInfo             *freeTrialInfo `json:"freeTrialInfo"`
	OverageCap                int            `json:"overageCap"`
	OverageRate               float64        `json:"overageRate"`
}

// freeTrialInfo 表示免费试用信息
type freeTrialInfo struct {
	FreeTrialStatus           string  `json:"freeTrialStatus"`
	FreeTrialExpiry           float64 `json:"freeTrialExpiry"`
	UsageLimit                int     `json:"usageLimit"`
	CurrentUsage              int     `json:"currentUsage"`
	UsageLimitWithPrecision   float64 `json:"usageLimitWithPrecision"`
	CurrentUsageWithPrecision float64 `json:"currentUsageWithPrecision"`
}

// subscriptionInfo 表示订阅信息
type subscriptionInfo struct {
	SubscriptionTitle            string `json:"subscriptionTitle"`
	Type                         string `json:"type"`
	OverageCapability            string `json:"overageCapability"`
	UpgradeCapability            string `json:"upgradeCapability"`
	SubscriptionManagementTarget string `json:"subscriptionManagementTarget"`
}

// userInfo 表示用户信息
type userInfo struct {
	Email  string `json:"email"`
	UserId string `json:"userId"`
}

// convert 将 kiroUsageResp 转换为 LimitRule 切片
func (r *kiroUsageResp) convert() []*limitrule.LimitRule {
	if len(r.UsageBreakdownList) == 0 {
		return nil
	}

	rules := make([]*limitrule.LimitRule, 0, len(r.UsageBreakdownList))
	for _, item := range r.UsageBreakdownList {
		// 优先使用精确浮点值，若为 0 则回退到整数字段
		total := item.UsageLimitWithPrecision
		used := item.CurrentUsageWithPrecision

		rule := &limitrule.LimitRule{
			SourceType:      limitrule.SourceTypeToken,
			TimeGranularity: limitrule.GranularityMonth,
			WindowSize:      1,
			Total:           total,
			Used:            used,
			Remain:          total - used,
		}
		rule.CalculateWindowTime()
		rules = append(rules, rule)
	}
	return rules
}

// Models 返回默认支持的模型列表
func (p *kiroProvider) Models(_ context.Context) ([]string, error) {
	models := make([]string, 0, len(ModelList))
	for _, k := range ModelList {
		models = append(models, k)
	}
	return models, nil
}

// ListModels 获取当前凭证支持的模型列表
// kiro 不提供动态模型列表接口，直接返回默认模型列表
func (p *kiroProvider) ListModels(ctx context.Context, _ credentials.Credentials) ([]string, error) {
	return p.Models(ctx)
}

// GetUsage 获取当前凭证的今日已使用量
func (p *kiroProvider) GetUsage(ctx context.Context, creds credentials.Credentials) ([]*limitrule.LimitRule, error) {
	resp, err := p.send(ctx, creds)
	if err != nil {
		return nil, errors.Annotate(err, "send get usage limits request failed")
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Annotate(err, "read get usage limits response failed")
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return nil, &providers.HTTPError{
			ErrorType:     providers.ErrorTypeRateLimit,
			ErrorCode:     providers.ErrorCodeRateLimit,
			Message:       "rate limit exceeded",
			RawStatusCode: resp.StatusCode,
			RawBody:       respBody,
		}
	default:
		if resp.StatusCode != http.StatusOK {
			return nil, &providers.HTTPError{
				ErrorType:     providers.ErrorTypeServerError,
				ErrorCode:     resp.StatusCode,
				Message:       fmt.Sprintf("get usage limits failed, status=%d", resp.StatusCode),
				RawStatusCode: resp.StatusCode,
				RawBody:       respBody,
			}
		}
	}

	var result kiroUsageResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, errors.Annotatef(err, "parse get usage limits response failed, status=%d, body=%s",
			resp.StatusCode, string(respBody))
	}
	return result.convert(), nil
}

func (p *kiroProvider) send(ctx context.Context, creds credentials.Credentials) (*http.Response, error) {
	kiroCreds, ok := creds.(*kirocreds.Credentials)
	if !ok {
		return nil, errors.New("invalid credentials type")
	}
	rawURL := fmt.Sprintf(usageURL, kiroCreds.Region)
	if kiroCreds.AuthMethod == kirocreds.AuthMethodSocial && kiroCreds.ProfileArn != "" {
		rawURL += "&profileArn=" + kiroCreds.ProfileArn
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, errors.Annotate(err, "create get usage limits request failed")
	}
	for k, v := range p.options.headerBuilder() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+kiroCreds.AccessToken)
	req.Header.Set("amz-sdk-invocation-id", uuid.NewString())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, errors.Annotate(err, "get usage limits request failed")
	}
	return resp, nil
}
