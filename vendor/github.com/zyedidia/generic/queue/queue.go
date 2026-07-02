// Package queue provides an implementation of a First In First Out (FIFO)
// queue. The FIFO queue is implemented using the doubly-linked list from the
// 'list' package.
package queue

import (
	"github.com/zyedidia/generic/list"
)

// Queue is a simple First In First Out (FIFO) queue.
type Queue[T any] struct {
	list   *list.List[T]
	length int
}

// New returns an empty First In First Out (FIFO) queue.
func New[T any]() *Queue[T] {
	return &Queue[T]{
		list: list.New[T](),
	}
}

// Of returns a First In First Out (FIFO) queue that has been populated with
// values from an existing slice.
func Of[S ~[]E, E any](slice S) *Queue[E] {
	queue := New[E]()
	for _, value := range slice {
		queue.Enqueue(value)
	}
	return queue
}

// Len returns the number of items currently in the queue.
func (q *Queue[T]) Len() int {
	return q.length
}

// Enqueue inserts 'value' to the end of the queue.
func (q *Queue[T]) Enqueue(value T) {
	q.list.PushBack(value)
	q.length++
}

// Dequeue removes and returns the item at the front of the queue.
//
// A panic occurs if the queue is Empty.
func (q *Queue[T]) Dequeue() T {
	value, ok := q.TryDequeue()
	if !ok {
		panic("queue: tried to dequeue from an empty queue")
	}
	return value
}

// TryDequeue tries to remove and return the item at the front of the queue.
//
// If the queue is empty, then false is returned as the second return value.
func (q *Queue[T]) TryDequeue() (T, bool) {
	if q.Empty() {
		var zero T
		return zero, false
	}
	value := q.list.Front.Value
	q.list.Remove(q.list.Front)
	q.length--
	return value, true
}

// DequeueAll removes and returns all the items in the queue.
func (q *Queue[T]) DequeueAll() []T {
	slice := make([]T, q.length)
	for i := 0; i < len(slice); i++ {
		slice[i] = q.Dequeue()
	}
	return slice
}

// Peek returns the item at the front of the queue without removing it.
//
// A panic occurs if the queue is Empty.
func (q *Queue[T]) Peek() T {
	if q.Empty() {
		panic("queue: tried to peek an empty queue")
	}
	return q.list.Front.Value
}

// TryPeek tries to return the item at the front of the queue without removing it.
//
// If the queue is empty, then false is returned as the second return value.
func (q *Queue[T]) TryPeek() (T, bool) {
	if q.Empty() {
		var zero T
		return zero, false
	}
	return q.list.Front.Value, true
}

// PeekAll returns all the items in the queue without removing them.
func (q *Queue[T]) PeekAll() []T {
	slice := make([]T, q.length)
	var index int
	q.list.Front.Each(func(val T) {
		slice[index] = val
		index++
	})
	return slice
}

// Empty returns true if the queue is empty.
func (q *Queue[T]) Empty() bool {
	return q.list.Front == nil
}

// Clear empties the queue, resetting it to zero elements.
func (q *Queue[T]) Clear() {
	q.length = 0
	q.list = list.New[T]()
}

// Copy returns a shallow copy of this queue.
func (q *Queue[T]) Copy() *Queue[T] {
	return Of(q.PeekAll())
}

// Each calls 'fn' on every item in the queue, starting with the least
// recently pushed element.
func (q *Queue[T]) Each(fn func(t T)) {
	q.list.Front.Each(fn)
}
