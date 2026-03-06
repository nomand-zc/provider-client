package httpclient

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/nomand-zc/provider-client/log"
)

// LoggingMiddleware 是内置的日志中间件，可直接传入 WithMiddleware 使用。
// 记录请求的 URL、Header、Body，以及响应的状态码、Header 和耗时。
// 响应 Body 不做读取，避免影响流式响应等场景。
var LoggingMiddleware RoundTripperMiddleware = func(next http.RoundTripper) http.RoundTripper {
	return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		start := time.Now()

		// ---- 记录请求信息 ----
		reqBody, err := readAndRestoreBody(req.Body)
		if err != nil {
			log.Warnf("[HTTPClient] 读取请求 Body 失败: %v", err)
		}
		// 无论读取是否成功，都写回 Body（失败时写回空，避免业务层拿到已关闭的流）
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
		log.Infof("[HTTPClient] --> %s %s\n  请求Header: %v\n  请求Body: %s",
			req.Method, req.URL.String(), req.Header, reqBody)

		resp, err := next.RoundTrip(req)

		elapsed := time.Since(start)

		// ---- 请求失败 ----
		if err != nil {
			log.Errorf("[HTTPClient] <-- %s %s 耗时=%s 错误=%v",
				req.Method, req.URL.String(), elapsed, err)
			return nil, err
		}

		// ---- 记录响应信息（不读取 Body，避免影响流式响应）----
		log.Infof("[HTTPClient] <-- %s %s 状态=%d 耗时=%s\n  响应Header: %v",
			req.Method, req.URL.String(), resp.StatusCode, elapsed, resp.Header)

		return resp, nil
	})
}

// readAndRestoreBody 读取 ReadCloser 中的全部内容并返回字节切片。
// 若 body 为 nil 或 http.NoBody 则返回空切片。
func readAndRestoreBody(body io.ReadCloser) ([]byte, error) {
	if body == nil || body == http.NoBody {
		return []byte{}, nil
	}
	defer body.Close()
	return io.ReadAll(body)
}
