package parser

import (
	"context"
	"sync"

	"github.com/nomand-zc/provider-client/providers"
)

// PayloadParser 定义事件流消息解析器接口
// 每种消息类型+事件类型的组合对应一个 PayloadParser 实现
type PayloadParser interface {
	// MessageType 返回解析器处理的消息类型（event/error/exception）
	MessageType() string
	// EventType 返回解析器处理的事件类型（仅当 MessageType 为 event 时有意义，否则返回空字符串）
	EventType() string
	// Parse 解析事件流消息并转换为通用响应格式
	Parse(ctx context.Context, msg *StreamMessage, opts ...OptionFunc) (*providers.Response, error)
}

// OptionFunc 定义解析器运行时参数选项函数
type OptionFunc func(*ParseOption)

// WithToolCallIndexManager 设置ToolCall索引管理器选项
func WithToolCallIndexManager(manager *ToolCallIndexManager) OptionFunc {
	return func(opt *ParseOption) {
		opt.ToolCallIndexManager = manager
	}
}

// ParseOption 定义解析器运行时参数选项
type ParseOption struct {
	// ToolCallIndexManager 管理ToolCall索引状态，确保每个GenerateContentStream调用有自己的索引计数器
	ToolCallIndexManager *ToolCallIndexManager
}

// ToolCallIndexManager 管理ToolCall索引状态
type ToolCallIndexManager struct {
	toolUseIndexMap   map[string]int // toolUseId -> index 映射
	toolUseIndexMutex sync.Mutex     // 并发安全锁
}

// NewToolCallIndexManager 创建新的ToolCall索引管理器
func NewToolCallIndexManager() *ToolCallIndexManager {
	return &ToolCallIndexManager{
		toolUseIndexMap: make(map[string]int),
	}
}

// GetToolCallIndex 获取或分配ToolCall索引
func (m *ToolCallIndexManager) GetToolCallIndex(toolUseId string) int {
	m.toolUseIndexMutex.Lock()
	defer m.toolUseIndexMutex.Unlock()

	if index, exists := m.toolUseIndexMap[toolUseId]; exists {
		return index
	}

	// 分配新的索引
	index := len(m.toolUseIndexMap)
	m.toolUseIndexMap[toolUseId] = index
	return index
}
