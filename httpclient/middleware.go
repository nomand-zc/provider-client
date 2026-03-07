package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/utils"
)

type requestLog struct {
	URL       string      `json:"url"`
	Method    string      `json:"method"`
	ReqHeader http.Header `json:"req_header"`
	ReqBody   []byte      `json:"req_body,omitempty"`

	RespCode   *int        `json:"resp_code,omitempty"`
	RespHeader http.Header `json:"resp_header,omitempty"`
	RespBody   []byte      `json:"resp_body,omitempty"`

	Err     error         `json:"err,omitempty"`
	Elapsed time.Duration `json:"elapsed,omitempty"`
}

// String 实现 Stringer 接口，按 curl 请求格式 + HTTP 响应格式输出
func (l *requestLog) String() string {
	var buf bytes.Buffer
	method := utils.If(l.Method == "", "GET", l.Method)
	// ---- curl 命令部分 ----
	buf.WriteString(fmt.Sprintf("\n====== 请求 =====:\ncurl -v -i -X  %s \\\n", method))
	for key, vals := range l.ReqHeader {
		val := strings.Join(vals, ", ")
		buf.WriteString(fmt.Sprintf(" -H '%s: %s' \\\n", key, val))
	}
	if len(l.ReqBody) > 0 {
		buf.WriteString(fmt.Sprintf(" -d '%s' \\\n", l.ReqBody))
	}
	if l.URL != "" {
		buf.WriteString(fmt.Sprintf(" '%s'", l.URL))
	}

	// ---- HTTP 响应部分 ----
	buf.WriteString("\n\n===== 响应: =====:\n")
	if l.RespCode != nil {
		buf.WriteString(fmt.Sprintf("HTTP %d", *l.RespCode))
	} else {
		buf.WriteString("HTTP ???\n")
	}
	if l.Elapsed > 0 {
		buf.WriteString(fmt.Sprintf(" | elapsed=%s(ms)", l.Elapsed))
	}
	if l.Err != nil {
		buf.WriteString(fmt.Sprintf(" | err=%v", l.Err))
	}
	for key, vals := range l.RespHeader {
		val := strings.Join(vals, ", ")
		buf.WriteString(fmt.Sprintf("\n%s: %s", key, val))
	}
	if len(l.RespBody) > 0 {
		buf.WriteString(fmt.Sprintf("\n\nBody:\n%s", l.RespBody))
	}

	return buf.String()
}

// LoggingMiddleware 是内置的日志中间件，可直接传入 WithMiddleware 使用。
// 记录请求的 URL、Header、Body，以及响应的状态码、Header 和耗时。
// 响应 Body 不做读取，避免影响流式响应等场景。
var LoggingMiddleware RoundTripperMiddleware = func(next http.RoundTripper) http.RoundTripper {
	return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		start := time.Now()

		// ---- 记录请求信息 ----
		reqBody, readBodyErr := readAndRestoreBody(req.Body)
		// 无论读取是否成功，都写回 Body（失败时写回空，避免业务层拿到已关闭的流）
		req.Body = io.NopCloser(bytes.NewReader(reqBody))

		reqLog := requestLog{
			URL:       req.URL.String(),
			Method:    req.Method,
			ReqHeader: req.Header,
			ReqBody:   reqBody,
			Err:       readBodyErr,
		}
		ctx := req.Context()
		if readBodyErr != nil {
			log.WarnContextf(ctx, "[HTTPClient-LoggingMiddleware]: %v", reqLog.String())
		}

		resp, err := next.RoundTrip(req)

		// ---- 记录响应信息 ----
		reqLog.Elapsed = time.Since(start)
		if resp != nil {
			reqLog.RespCode = &resp.StatusCode
			reqLog.RespHeader = resp.Header
			if EnabledPrintRespBody(ctx) {
				respBody, readBodyErr := readAndRestoreBody(resp.Body)
				reqLog.RespBody = respBody
				reqLog.Err = utils.If(reqLog.Err != nil, reqLog.Err, readBodyErr)
				// 无论读取是否成功，都写回 Body（失败时写回空，避免业务层拿到已关闭的流）
				resp.Body = io.NopCloser(bytes.NewReader(respBody))
			}
		}

		// ---- 请求失败 ----
		if err != nil {
			reqLog.Err = err
			log.ErrorContextf(ctx, "[HTTPClient-LoggingMiddleware]: %s", reqLog.String())
			return nil, err
		}

		// ---- 记录响应信息（不读取 Body，避免影响流式响应）----
		log.Infof("[HTTPClient-LoggingMiddleware]: %s", reqLog.String())

		return resp, nil
	})
}

// readAndRestoreBody 读取 ReadCloser 中的全部内容并返回字节切片。
// 若 body 为 nil 或 http.NoBody 则返回空切片。
func readAndRestoreBody(body io.ReadCloser) ([]byte, error) {
	if body == nil || body == http.NoBody {
		return nil, nil
	}
	defer body.Close()
	reqBody, err := io.ReadAll(body)
	return reqBody, errors.Annotate(err, "读取 Body 失败")
}
