package kiro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/juju/errors"
	"github.com/nomand-zc/provider-client/credentials"
	kirocreds "github.com/nomand-zc/provider-client/credentials/kiro"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
)

const (
	socailRefreshURL = "https://prod.%s.auth.desktop.kiro.dev/refreshToken"
	idcRefreshURL    = "https://oidc.%s.amazonaws.com/token"

	authMethodSocial = "social"
)

// Refresh 刷新令牌
func (r *kiroProvider) Refresh(ctx context.Context, creds credentials.Credentials) (
	credentials.Credentials, error) {
	kiroCreds, ok := creds.(*kirocreds.Credentials)
	if !ok {
		return nil, errors.New("invalid credentials type")
	}

	if kiroCreds.AuthMethod == authMethodSocial {
		return r.refreshSocialToken(ctx, kiroCreds)
	}

	return r.refreshIDCToken(ctx, kiroCreds)
}

type tokenRefreshResp struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"` // 刷新后可能返回新的 refreshToken
	ExpiresIn    int    `json:"expiresIn"`    // Token 有效期（秒），用于计算 expiresAt
	ProfileArn   string `json:"profileArn"`

	Error string `json:"error"` // 错误码
}

func (r *kiroProvider) refreshSocialToken(ctx context.Context, creds *kirocreds.Credentials) (*kirocreds.Credentials, error) {
	refreshURL := fmt.Sprintf(socailRefreshURL, creds.Region)
	reqBody := map[string]string{"refreshToken": creds.RefreshToken}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.Annotate(err, "marshal refresh request failed")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, errors.Annotate(err, "create refresh request failed")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, errors.Annotate(err, "kiro social refresh request failed")
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	log.DebugContext(ctx, "kiro social refresh response statusCode=%d, respBody=%s",
		resp.StatusCode, string(respBody))
	if err != nil {
		return nil, errors.Annotate(err, "read kiro social refresh response failed")
	}

	var result tokenRefreshResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, errors.Annotatef(err, "parse kiro social refresh response failed, status=%d, body=%s",
			resp.StatusCode, string(respBody))
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return nil, &providers.HTTPError{
			ErrorType:     providers.ErrorTypeRateLimit,
			ErrorCode:     resp.StatusCode,
			Message:       "kiro social refresh rate limit",
			RawStatusCode: resp.StatusCode,
			RawBody:       respBody,
		}
	default:
		if resp.StatusCode != http.StatusOK {
			return nil, providers.ErrInvalidGrant
		}
	}

	newCreds := creds.Clone()
	newCreds.AccessToken = result.AccessToken
	newCreds.RefreshToken = result.RefreshToken
	newCreds.ProfileArn = result.ProfileArn
	expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	newCreds.ExpiresAt = &expiresAt

	return newCreds, nil
}

func (r *kiroProvider) refreshIDCToken(ctx context.Context, creds *kirocreds.Credentials) (*kirocreds.Credentials, error) {
	refreshURL := fmt.Sprintf(idcRefreshURL, creds.IDCRegion)
	reqBody := map[string]string{
		"refreshToken": creds.RefreshToken,
		"clientId":     creds.ClientID,
		"clientSecret": creds.ClientSecret,
		"grantType":    "refresh_token",
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.Annotate(err, "marshal kiro IDC refresh request failed")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, errors.Annotate(err, "create kiro IDC refresh request failed")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, errors.Annotate(err, "kiro IDC refresh request failed")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Annotate(err, "read kiro IDC refresh response failed")
	}

	var result tokenRefreshResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, errors.Annotatef(err, "parse kiro IDC refresh response failed, status=%d, body=%s",
			resp.StatusCode, string(respBody))
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return nil, &providers.HTTPError{
			ErrorType:     providers.ErrorTypeRateLimit,
			ErrorCode:     resp.StatusCode,
			Message:       "kiro IDC refresh rate limit",
			RawStatusCode: resp.StatusCode,
			RawBody:       respBody,
		}
	default:
		if resp.StatusCode != http.StatusOK {
			return nil, providers.ErrInvalidGrant
		}
	}

	res := *creds
	res.AccessToken = result.AccessToken
	res.RefreshToken = result.RefreshToken
	res.ProfileArn = result.ProfileArn
	expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	res.ExpiresAt = &expiresAt

	return &res, nil
}
