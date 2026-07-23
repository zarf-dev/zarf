package partybus

import (
	"container/list"
)

func newQueue() (chan<- Event, <-chan Event) {
	send := make(chan Event, 1)
	receive := make(chan Event, 1)
	go manageQueue(send, receive)
	return send, receive
}

func manageQueue(send <-chan Event, receive chan<- Event) {
	queue := list.New()
	for {
		if front := queue.Front(); front == nil {
			if send == nil {
				close(receive)
				return
			}
			value, ok := <-send
			if !ok {
				close(receive)
				return
			}
			queue.PushBack(value)
		} else {
			select {
			case receive <- front.Value.(Event):
				queue.Remove(front)
			case value, ok := <-send:
				if ok {
					queue.PushBack(value)
				} else {
					send = nil
				}
			}
		}
	}
}
