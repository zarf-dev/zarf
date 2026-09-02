package partybus

var nextID int64

// type EventCallback func(Event)

var _ Unsubscribable = (*Subscription)(nil)

type SubscriptionId int64

type Subscription struct {
	id  SubscriptionId
	bus *Bus
	// allCallback    []EventCallback
	// selectCallback map[EventType]EventCallback
	sender     chan<- Event
	receiver   <-chan Event
	eventTypes []EventType
}

func newSubscription(bus *Bus, eventKinds []EventType) *Subscription {
	nextID++
	sender, receiver := newQueue()
	return &Subscription{
		id:         SubscriptionId(nextID),
		bus:        bus,
		sender:     sender,
		receiver:   receiver,
		eventTypes: eventKinds,
		// allCallback:    make([]EventCallback, 0),
		// selectCallback: make(map[EventType]EventCallback),
	}
}

func (s *Subscription) Unsubscribe() error {
	return s.bus.Unsubscribe(s)
}

// func (s *Subscription) Register(callback EventCallback, eTypes ...EventType) error {

// 	if len(eTypes) == 0 {
// 		s.allCallback = append(s.allCallback, callback)
// 		go func() {
// 			for event := range s.receiver {
// 				for c := range s.allCallback {
// 					s.allCallback(event)
// 				}
// 			}
// 		}()
// 	} else {
// 		s.selectCallback[eType]
// 	}

// 	return nil
// }

func (s *Subscription) Events() <-chan Event {
	return s.receiver
}
