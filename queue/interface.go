package queue

import "context"

type Reader[T any] interface {
	Closed() bool
	Read(ctx context.Context) (T, error)
	Len() int
}

type Writer[T any] interface {
	Closed() bool
	Write(ctx context.Context, item T) error
}

type Queue[T any] interface {
	Reader[T]
	Writer[T]
	Closed() bool
	Close() error
}