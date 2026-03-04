package client

import "net/http"

// HTTPClient is the interface for the HTTP client.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// HTTPClientNewFunc is the function type for creating a new HTTP client.
type HTTPClientNewFunc func(opts ...HTTPClientOption) HTTPClient

// DefaultNewHTTPClient is the default HTTP client for Anthropic.
var DefaultNewHTTPClient HTTPClientNewFunc = func(opts ...HTTPClientOption) HTTPClient {
	options := &HTTPClientOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return &http.Client{
		Transport: options.Transport,
	}
}

// HTTPClientOption is the option for the HTTP client.
type HTTPClientOption func(*HTTPClientOptions)

// WithHTTPClientName is the option for the HTTP client name.
func WithHTTPClientName(name string) HTTPClientOption {
	return func(options *HTTPClientOptions) {
		options.Name = name
	}
}

// WithHTTPClientTransport is the option for the HTTP client transport.
func WithHTTPClientTransport(transport http.RoundTripper) HTTPClientOption {
	return func(options *HTTPClientOptions) {
		options.Transport = transport
	}
}

// HTTPClientOptions is the options for the HTTP client.
type HTTPClientOptions struct {
	Name      string
	Transport http.RoundTripper
}
