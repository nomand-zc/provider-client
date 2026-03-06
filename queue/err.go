package queue

import "errors"

var (
	// ErrQueueClosed 表示队列已关闭的错误
	ErrQueueClosed = errors.New("queue is closed")
	// ErrQueueFull 表示队列缓冲区已满的错误
	ErrQueueFull = errors.New("queue is full")
)

// IsClosedError 判断错误是否为队列关闭错误
func IsClosedError(err error) bool {
	return errors.Is(err, ErrQueueClosed)
}

// IsFullError 判断错误是否为队列满错误
func IsFullError(err error) bool {
	return errors.Is(err, ErrQueueFull)
}
