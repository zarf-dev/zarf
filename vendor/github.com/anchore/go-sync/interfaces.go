package sync

// Provider returns a single item
type Provider[T any] interface {
	Get() T
}

// Iterable provides a function to iterate a series of values
type Iterable[T any] interface {
	// Seq provides an iter.Seq compatible iterator
	Seq(fn func(value T) bool)
}

// Appender to allow values to be appended
type Appender[T any] interface {
	Append(value T)
}

// Collection is a generic collection of values, which can be added to, removed from, and provide a length
type Collection[T any] interface {
	Iterable[T]
	Appender[T]
	Remove(value T)
	Contains(value T) bool
	Len() int
}

// Queue is a generic queue interface
type Queue[T any] interface {
	Enqueue(value T)
	Dequeue() (value T, ok bool)
}

// Stack is a generic stack interface
type Stack[T any] interface {
	Push(value T)
	Pop() (value T, ok bool)
	Peek() (value T, ok bool)
}
