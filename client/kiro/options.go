package kiro

import "maps"

const (
	AuthMethodSocial = "social"
)

var defaultOptions = Options{
	headers: map[string]string{
		"Content-Type":           "application/json",
		"Accept":                 "application/json",
		"amz-sdk-request":        "attempt=1; max=1",
		"x-amzn-kiro-agent-mode": "vibe",
		"x-amz-user-agent":       "aws-sdk-js/1.0.0 KiroIDE-0.8.140",
		"User-Agent":             "aws-sdk-js/1.0.0 ua/2.1 api/codewhispererruntime#1.0.0 m/E KiroIDE-0.8.140",
		"Connection":             "close",
	},
	url:           "https://q.%s.amazonaws.com/generateAssistantResponse",
	defaultRegion: "us-east-1",
}

type Options struct {
	url           string
	headers       map[string]string
	defaultRegion string
}

type Option func(*Options)

func WithURL(url string) Option {
	return func(o *Options) {
		o.url = url
	}
}

func WithHeaders(headers map[string]string) Option {
	return func(o *Options) {
		maps.Copy(o.headers, headers)
	}
}

func WithDefaultRegion(region string) Option {
	return func(o *Options) {
		o.defaultRegion = region
	}
}
