package partybus

import (
	"fmt"
	"sync"
)

var ErrUnsubscribe = fmt.Errorf("unable to find subscription to unsubscribe")

type Responder interface {
	RespondsTo() []EventType
}

type Handler interface {
	Handle(Event) error
}

type Publisher interface {
	Publish(Event)
}

type Subscriber interface {
	Subscribe(...EventType) *Subscription
}

type Unsubscribable interface {
	Unsubscribe() error
}

type Bus struct {
	selectSubs map[EventType][]*Subscription
	fullSubs   []*Subscription
	allSubs    []*Subscription
	lock       sync.RWMutex
}

func NewBus() *Bus {
	return &Bus{
		selectSubs: make(map[EventType][]*Subscription),
		fullSubs:   make([]*Subscription, 0),
		allSubs:    make([]*Subscription, 0),
	}
}

func (bus *Bus) Publish(event Event) {
	bus.lock.RLock()
	defer bus.lock.RUnlock()

	// send to full subscribers
	for _, sub := range bus.fullSubs {
		sub.sender <- event
	}

	// send to select subscribers
	if subs, ok := bus.selectSubs[event.Type]; ok {
		for _, sub := range subs {
			sub.sender <- event
		}
	}
}

func (bus *Bus) Subscribe(eTypes ...EventType) *Subscription {
	bus.lock.Lock()
	defer bus.lock.Unlock()

	sub := newSubscription(bus, eTypes)

	if len(eTypes) == 0 {
		// subscribe to all events
		bus.fullSubs = append(bus.fullSubs, sub)
	} else {
		// subscribe to select events
		for _, eType := range eTypes {
			bus.selectSubs[eType] = append(bus.selectSubs[eType], sub)
		}
	}

	bus.allSubs = append(bus.allSubs, sub)

	return sub
}

func (bus *Bus) Unsubscribe(sub *Subscription) error {
	bus.lock.Lock()
	defer bus.lock.Unlock()

	var found bool

	for _, eType := range sub.eventTypes {
		if _, ok := bus.selectSubs[eType]; ok {
			// unsubscribe from select events
			for idx, eSub := range bus.selectSubs[eType] {
				if eSub.id == sub.id {
					found = true
					bus.selectSubs[eType] = append(bus.selectSubs[eType][:idx], bus.selectSubs[eType][idx+1:]...)
				}
			}
		}
	}

	// unsubscribe from all events
	for idx, eSub := range bus.fullSubs {
		if eSub.id == sub.id {
			found = true
			bus.fullSubs = append(bus.fullSubs[:idx], bus.fullSubs[idx+1:]...)
		}
	}

	// close the sender channel
	for idx, eSub := range bus.allSubs {
		if eSub.id == sub.id {
			found = true
			close(eSub.sender)
			bus.allSubs = append(bus.fullSubs[:idx], bus.allSubs[idx+1:]...)
		}
	}

	if !found {
		return ErrUnsubscribe
	}
	return nil
}

func (bus *Bus) Close() {
	bus.lock.Lock()
	defer bus.lock.Unlock()

	for _, sub := range bus.allSubs {
		close(sub.sender)
	}

	bus.selectSubs = make(map[EventType][]*Subscription)
	bus.fullSubs = make([]*Subscription, 0)
	bus.allSubs = make([]*Subscription, 0)
}
