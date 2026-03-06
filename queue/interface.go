package queue

type Reader[T any] interface {
	Closed() bool
	Read() (T, error)
	Len() int
}

type Writer[T any] interface {
	Closed() bool
	Write(T) error
}

type Queue[T any] interface {
	Reader[T]
	Writer[T]
	Closed() bool
	Close() error
}