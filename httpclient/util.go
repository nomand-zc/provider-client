package httpclient

import "context"

type printRespBodyKey string

// EnablePrintRespBody 启用打印响应体
func EnablePrintRespBody(ctx context.Context) context.Context {
	return context.WithValue(ctx, printRespBodyKey("print_resp_body"), true)
}

// EnabledPrintRespBody 是否启用打印响应体
func EnabledPrintRespBody(ctx context.Context) bool {
	return ctx.Value(printRespBodyKey("print_resp_body")) != nil
}
