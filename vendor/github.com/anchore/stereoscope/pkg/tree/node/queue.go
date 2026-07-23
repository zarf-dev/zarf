package node

type Queue struct {
	head int
	data []Node
}

func (q *Queue) Size() int {
	return len(q.data) - q.head
}

func (q *Queue) Enqueue(n Node) {
	if len(q.data) == cap(q.data) && q.head > 0 {
		l := q.Size()
		copy(q.data, q.data[q.head:])
		q.head = 0
		q.data = append(q.data[:l], n)
	} else {
		q.data = append(q.data, n)
	}
}

func (q *Queue) Dequeue() Node {
	if q.Size() == 0 {
		return nil
	}

	var node Node
	node, q.data[q.head] = q.data[q.head], nil
	q.head++

	if q.Size() == 0 {
		q.head = 0
		q.data = q.data[:0]
	}

	return node
}

func (q *Queue) Reset() {
	q.head = 0
	q.data = q.data[:0]
}
