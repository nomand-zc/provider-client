package kiro

import (
	"github.com/nomand-zc/provider-client/providers"
	"github.com/nomand-zc/provider-client/providers/tiktoken"
)

const (
	DefaultRegion = "us-east-1"
)

var defaultOptions = Options{
	url:           "https://q.%s.amazonaws.com/generateAssistantResponse",
	headerBuilder: DefaultHeaderBuilder,
	defaultRegion: DefaultRegion,
}

func init() {
	tokenConter, err := tiktoken.New("claude-sonnet-4.6")
	if err != nil {
		panic(err)
	}
	defaultOptions.tokenConter = tokenConter
}

// HeaderBuilder builds headers for the request.
type HeaderBuilder func() map[string]string

// DefaultHeaderBuilder returns the default headers.
func DefaultHeaderBuilder() map[string]string {
	return map[string]string{
		"Content-Type":    "application/json",
		"Accept":          "application/json",
		"amz-sdk-request": "attempt=1; max=1",

		// vide
		"x-amzn-kiro-agent-mode": "vibe",

		"x-amz-user-agent": "aws-sdk-js/1.0.0 KiroIDE-0.10.78",
		// "x-amz-user-agent": "aws-sdk-js/1.0.18 KiroIDE-0.2.13-66c23a8c5d15afabec89ef9954ef52a119f10d369df04d548fc6c1eac694b0d1",

		"User-Agent": "aws-sdk-js/1.0.0 ua/2.1 api/codewhispererruntime#1.0.0 m/E KiroIDE-0.10.78",
		// "User-Agent": "aws-sdk-js/1.0.18 ua/2.1 os/darwin#25.0.0 lang/js md/nodejs#20.16.0 api/codewhispererstreaming#1.0.18 m/E KiroIDE-0.2.13-66c23a8c5d15afabec89ef9954ef52a119f10d369df04d548fc6c1eac694b0d1",
	}
}

// Options contains the options for the client.
type Options struct {
	url           string
	headerBuilder HeaderBuilder
	defaultRegion string
	tokenConter   providers.TokenCounter
}

// Option is a function that sets an option.
type Option func(*Options)

// WithURL sets the URL.
func WithURL(url string) Option {
	return func(o *Options) {
		o.url = url
	}
}

// WithDefaultRegion sets the default region.
func WithDefaultRegion(region string) Option {
	return func(o *Options) {
		o.defaultRegion = region
	}
}
