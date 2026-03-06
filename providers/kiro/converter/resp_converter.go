package converter

import (
	"context"
	"encoding/json"
	"io"

	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
	"github.com/nomand-zc/provider-client/providers/kiro/converter/parser"
	"github.com/nomand-zc/provider-client/queue"
)

const (
	defaultBufferSize = 1024
	defaultQueueSize  = 100
)

// ProcessEventStream 处理事件流
func ProcessEventStream(ctx context.Context, reader io.Reader) (queue.Reader[*parser.EventStreamMessage], error) {
	buf := make([]byte, defaultBufferSize)
	chainQueue := queue.NewChainQueue[*parser.EventStreamMessage](defaultQueueSize)
	var totalReadBytes int

	go func(){
		defer chainQueue.Close()
		for {
			n, err := reader.Read(buf)
			totalReadBytes += n
			if n > 0 {
				// 解析事件流
				events, parseErr := parser.DefaultRobustParser.ParseStream(buf[:n])

				if parseErr != nil {
					log.Warnf("符合规范的解析器处理失败, err: %v, read_bytes: %d",
						parseErr, n)
				}
				for _, event := range events {
					chainQueue.Write(event)
				}
			}

			if err != nil {
				if err == io.EOF {
					log.Debugf("响应流结束, total_read_bytes: %d", totalReadBytes)
				} else {
					log.Errorf("读取响应流时发生错误, err: %v, total_read_bytes: %d",
						err, totalReadBytes)
				}
				break
			}
		}
	}()
	

	// 直传模式：无需冲刷剩余文本
	return chainQueue, nil
}


// ConvertResponse 将 Kiro CodeWhisperer 响应转换为通用响应格式
func ConvertResponse(_ context.Context, resp *parser.EventStreamMessage) (
	*providers.Response, error) {
	jsonData, _ := json.Marshal(resp)
	log.Debugf("kiro response: %s", string(jsonData))
	return nil, nil
}
