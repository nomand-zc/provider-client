package httpclient

import (
	"net/http"
	"time"
)

// RoundTripperMiddleware 中间件类型，基于标准 http.RoundTripper。
// 每个中间件接收下一个 RoundTripper，返回一个新的 RoundTripper。
type RoundTripperMiddleware func(next http.RoundTripper) http.RoundTripper

// RoundTripperFunc 将普通函数适配为 http.RoundTripper 接口。
type RoundTripperFunc func(req *http.Request) (*http.Response, error)

// HTTPClient 是 HTTP 客户端接口。
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// RoundTrip implements http.RoundTripper.
func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// New 创建一个带中间件的标准 *http.Client。
// 中间件按注册顺序执行（第一个注册的最先执行，洋葱模型）。
func New(opts ...Option) HTTPClient {
	options := &Options{
		Transport: http.DefaultTransport,
	}
	for _, opt := range opts {
		opt(options)
	}
	// 逆序包装，使第一个注册的中间件最先执行
	transport := options.Transport
	for i := len(options.Middlewares) - 1; i >= 0; i-- {
		transport = options.Middlewares[i](transport)
	}
	return &http.Client{
		Transport: transport,
		Timeout:   options.Timeout,
	}
}

// Option 是创建 HTTP 客户端的选项函数。
type Option func(*Options)

// Options 是 HTTP 客户端的配置项。
type Options struct {
	Name string
	// Base 是最底层的 RoundTripper，默认为 http.DefaultTransport。
	Transport http.RoundTripper
	// Middlewares 是中间件列表，按注册顺序执行。
	Middlewares []RoundTripperMiddleware
	// Timeout 是请求超时时间，0 表示不限制。
	Timeout time.Duration
}

// WithHTTPClientName is the option for the HTTP client name.
func WithHTTPClientName(name string) Option {
	return func(options *Options) {
		options.Name = name
	}
}

// WithTransport 设置底层 RoundTripper（默认为 http.DefaultTransport）。
func WithTransport(transport http.RoundTripper) Option {
	return func(o *Options) {
		o.Transport = transport
	}
}

// WithMiddleware 追加一个或多个中间件。
func WithMiddleware(middlewares ...RoundTripperMiddleware) Option {
	return func(o *Options) {
		o.Middlewares = append(o.Middlewares, middlewares...)
	}
}

// WithTimeout 设置请求超时时间。
func WithTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.Timeout = timeout
	}
}
