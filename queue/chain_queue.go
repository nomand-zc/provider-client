package queue

import (
	"sync/atomic"
)

type queue[T any] struct {
	c      chan T
	closed uint32 // 使用原子操作保证并发安全
}

// NewChanQueue 创建一个基于channel的队列
func NewChanQueue[T any](bufferSize int) Queue[T] {
	return &queue[T]{
		c: make(chan T, bufferSize),
	}
}

func (q *queue[T]) Len() int {
	return len(q.c)
}

func (q *queue[T]) Read() (T, error) {
	var zero T
	
	item, ok := <-q.c
	if !ok {
		return zero, ErrQueueClosed
	}
	return item, nil
}

func (q *queue[T]) Write(item T) error {
	// 使用读锁快速检查关闭状态
	if atomic.LoadUint32(&q.closed) == 1 {
		return ErrQueueClosed
	}
	
	q.c <- item
	return nil
}

func (q *queue[T]) Close() error {
	
	// 使用原子操作确保只关闭一次
	if atomic.CompareAndSwapUint32(&q.closed, 0, 1) {
		close(q.c)
		return nil
	}
	
	// 如果已经关闭，返回错误
	return ErrQueueClosed
}

func (q *queue[T]) Closed() bool {
	return atomic.LoadUint32(&q.closed) == 1
}