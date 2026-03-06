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

// usageLimitsResp 表示 getUsageLimits 接口的响应
type usageLimitsResp struct {
	UsageLimits *usageLimits `json:"usageLimits"`
	Error       string       `json:"error"`
	Message     string       `json:"message"`
}

type usageLimits struct {
	DailyLimit         int64  `json:"dailyLimit"`
	UsedToday          int64  `json:"usedToday"`
	RemainingToday     int64  `json:"remainingToday"`
	MonthlyLimit       int64  `json:"monthlyLimit"`
	UsedThisMonth      int64  `json:"usedThisMonth"`
	RemainingThisMonth int64  `json:"remainingThisMonth"`
	ResetTime          string `json:"resetTime"`
	AccountStatus      string `json:"accountStatus"`
	Tier               string `json:"tier"`
}

func (ul *usageLimits) convert() []*limitrule.LimitRule {
	// 日限额规则
	dailyRule := limitrule.LimitRule{
		SourceType:      limitrule.SourceTypeRequest,
		TimeGranularity: limitrule.GranularityDay,
		WindowSize:      1,
		Total:           float64(ul.DailyLimit),
		Used:            float64(ul.UsedToday),
		Remain:          float64(ul.RemainingToday),
	}
	dailyRule.CalculateWindowTime()

	// 月限额规则
	monthlyRule := limitrule.LimitRule{
		SourceType:      limitrule.SourceTypeRequest,
		TimeGranularity: limitrule.GranularityMonth,
		WindowSize:      1,
		Total:           float64(ul.MonthlyLimit),
		Used:            float64(ul.UsedThisMonth),
		Remain:          float64(ul.RemainingThisMonth),
	}
	monthlyRule.CalculateWindowTime()

	return []*limitrule.LimitRule{&dailyRule, &monthlyRule}
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
	case http.StatusUnauthorized:
		return nil, &providers.HTTPError{
			ErrorType:     providers.ErrorTypeUnauthorized,
			ErrorCode:     providers.ErrorCodeUnauthorized,
			Message:       "invalid or expired access token",
			RawStatusCode: resp.StatusCode,
			RawBody:       respBody,
		}
	case http.StatusForbidden:
		return nil, &providers.HTTPError{
			ErrorType:     providers.ErrorTypeForbidden,
			ErrorCode:     providers.ErrorCodeForbidden,
			Message:       "account temporarily suspended or insufficient permissions",
			RawStatusCode: resp.StatusCode,
			RawBody:       respBody,
		}
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
				ErrorCode:     providers.ErrorCodeServerError,
				Message:       fmt.Sprintf("get usage limits failed, status=%d", resp.StatusCode),
				RawStatusCode: resp.StatusCode,
				RawBody:       respBody,
			}
		}
	}

	var result usageLimitsResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, errors.Annotatef(err, "parse get usage limits response failed, status=%d, body=%s",
			resp.StatusCode, string(respBody))
	}
	if result.UsageLimits == nil {
		return nil, errors.New("get usage limits response missing usageLimits field")
	}
	return result.UsageLimits.convert(), nil
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
