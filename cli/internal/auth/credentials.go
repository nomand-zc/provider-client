package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nomand-zc/provider-client/credentials"
	kirocreds "github.com/nomand-zc/provider-client/credentials/kiro"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
)

// LoadCredentials 从文件中加载凭证
func LoadCredentials(providerName string, file string) (credentials.Credentials, error) {
	// 读取文件
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("读取凭证文件失败: %w", err)
	}

	var creds credentials.Credentials

	switch providerName {
	case "kiro":
		creds = kirocreds.NewCredentials(raw)
	default:
		return nil, fmt.Errorf("不支持的 provider: %q，支持的 provider 列表：%v", providerName, "kiro")
	}

	// 验证凭证，但允许过期凭证（ErrExpiresAtExpired 错误不视为失败）
	if err = creds.Validate(); err != nil && err != credentials.ErrExpiresAtExpired {
		return creds, fmt.Errorf("验证凭证失败: %w", err)
	}

	return creds, nil
}

// SaveCredentials 将凭证保存到文件
func SaveCredentials(creds credentials.Credentials, file string) error {
	if creds == nil {
		return fmt.Errorf("凭证不能为空")
	}
	if err := creds.Validate(); err != nil && err != credentials.ErrExpiresAtExpired {
		return fmt.Errorf("验证凭证失败: %w", err)
	}

	// 将刷新后的凭证写回文件
	credsJSON, err := json.MarshalIndent(creds, "", "    ")
	if err != nil {
		return fmt.Errorf("序列化凭证失败: %w", err)
	}
	if err := os.WriteFile(file, credsJSON, 0655); err != nil {
		return fmt.Errorf("写入凭证文件失败: %w", err)
	}

	return nil
}

// GetValidCredentials 获取有效的凭证
func GetValidCredentials(provider providers.Provider, dirPath string) (credentials.Credentials, error) {
	// 检查credFile是文件还是目录
	fileInfo, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("无法访问凭证路径: %w", err)
	}
	if !fileInfo.IsDir() {
		// 如果是文件，直接使用
		return GetCredentialsFromFile(provider, dirPath)
	}
	var finalCreds credentials.Credentials
	// 定义特殊错误用于提前终止遍历
	var foundCredentialError = fmt.Errorf("found valid credential")

	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if finalCreds != nil {
			// 已经找到有效凭证，提前终止遍历
			return foundCredentialError
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".json") {
			return nil
		}
		cred, err := GetCredentialsFromFile(provider, path)
		if err != nil {
			return nil
		}
		finalCreds = cred
		// 找到有效凭证，提前终止遍历
		return foundCredentialError
	})

	// 如果是我们预期的提前终止错误，忽略它
	if err != nil && err != foundCredentialError {
		return nil, err
	}

	return finalCreds, nil
}

func GetCredentialsFromFile(provider providers.Provider, file string) (credentials.Credentials, error) {
	// 构建凭证
	creds, err := LoadCredentials(provider.Name(), file)
	if err != nil {
		return nil, fmt.Errorf("构建凭证失败: %w", err)
	}

	// 检查凭证是否过期，如果过期则刷新
	if err := creds.Validate(); err != nil {
		if err != credentials.ErrExpiresAtExpired {
			return nil, fmt.Errorf("无效凭证: %w", err)
		}
		log.Infof("检测到凭证已过期，正在刷新...")
		refreshedCreds, err := provider.Refresh(context.Background(), creds)
		if err != nil {
			return nil, fmt.Errorf("刷新凭证失败: %w", err)
		}

		if err := SaveCredentials(refreshedCreds, file); err != nil {
			return nil, fmt.Errorf("保存刷新后的凭证失败: %w", err)
		}

		// TODO： 检查是否还有额度

		log.Infof("凭证刷新成功，已更新到文件: %s", file)
		creds = refreshedCreds
	}

	return creds, nil
}
