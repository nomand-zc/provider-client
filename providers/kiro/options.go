package kiro

const (
	DefaultRegion = "us-east-1"
)

var defaultOptions = Options{
	url:           "https://q.%s.amazonaws.com/generateAssistantResponse",
	headerBuilder: DefaultHeaderBuilder,
	defaultRegion: DefaultRegion,
}

// HeaderBuilder builds headers for the request.
type HeaderBuilder func() map[string]string

// DefaultHeaderBuilder returns the default headers.
func DefaultHeaderBuilder() map[string]string {
	return map[string]string{
		"Content-Type":           "application/json",
		"Accept":                 "application/json",
		"amz-sdk-request":        "attempt=1; max=1",
		"x-amzn-kiro-agent-mode": "vibe",
		"x-amz-user-agent":       "aws-sdk-js/1.0.0 KiroIDE-0.8.140",
		"User-Agent":             "aws-sdk-js/1.0.0 ua/2.1 api/codewhispererruntime#1.0.0 m/E KiroIDE-0.8.140",
	}
}

// Options contains the options for the client.
type Options struct {
	url           string
	headerBuilder HeaderBuilder
	defaultRegion string
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
