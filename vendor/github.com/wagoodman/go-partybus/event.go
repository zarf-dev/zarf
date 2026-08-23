package partybus

import "sync"

type EventType string

type Event struct {
	Type   EventType
	Source interface{}
	Value  interface{}
	Error  error
}

func Join(cs ...<-chan Event) <-chan Event {
	var wg sync.WaitGroup
	trunk := make(chan Event)

	read := func(c <-chan Event) {
		for n := range c {
			trunk <- n
		}
		wg.Done()
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go read(c)
	}

	go func() {
		wg.Wait()
		close(trunk)
	}()
	return trunk
}
