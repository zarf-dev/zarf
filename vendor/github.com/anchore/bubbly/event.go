package bubbly

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/go-partybus"
)

var (
	_ EventHandler = (*EventDispatcher)(nil)
	_ interface {
		EventHandler
		MessageListener
		HandleWaiter
	} = (*HandlerCollection)(nil)
)

type EventHandlerFn func(partybus.Event) ([]tea.Model, tea.Cmd)

type EventHandler interface {
	partybus.Responder
	// Handle optionally generates new models and commands in response to the given event. It might be that the event
	// has an effect on the system, but the model is managed by a sub-component, in which case no new model would be
	// returned but the Init() call on the managed model would return commands that should be executed in the context
	// of the application lifecycle.
	Handle(partybus.Event) ([]tea.Model, tea.Cmd)
}

type MessageListener interface {
	OnMessage(tea.Msg)
}

type HandleWaiter interface {
	Wait()
}

type EventDispatcher struct {
	dispatch map[partybus.EventType]EventHandlerFn
	types    []partybus.EventType
}

func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{
		dispatch: map[partybus.EventType]EventHandlerFn{},
	}
}

func (d *EventDispatcher) AddHandlers(handlers map[partybus.EventType]EventHandlerFn) {
	for t, h := range handlers {
		d.AddHandler(t, h)
	}
}

func (d *EventDispatcher) AddHandler(t partybus.EventType, fn EventHandlerFn) {
	d.dispatch[t] = fn
	d.types = append(d.types, t)
}

func (d EventDispatcher) RespondsTo() []partybus.EventType {
	return d.types
}

func (d EventDispatcher) Handle(e partybus.Event) ([]tea.Model, tea.Cmd) {
	if fn, ok := d.dispatch[e.Type]; ok {
		return fn(e)
	}
	return nil, nil
}

type HandlerCollection struct {
	handlers []EventHandler
}

func NewHandlerCollection(handlers ...EventHandler) *HandlerCollection {
	return &HandlerCollection{
		handlers: handlers,
	}
}

func (h *HandlerCollection) Append(handlers ...EventHandler) {
	h.handlers = append(h.handlers, handlers...)
}

func (h HandlerCollection) RespondsTo() []partybus.EventType {
	var ret []partybus.EventType
	for _, handler := range h.handlers {
		ret = append(ret, handler.RespondsTo()...)
	}
	return ret
}

func (h HandlerCollection) Handle(event partybus.Event) ([]tea.Model, tea.Cmd) {
	var (
		newModels []tea.Model
		newCmd    tea.Cmd
	)
	for _, handler := range h.handlers {
		mods, cmd := handler.Handle(event)
		newModels = append(newModels, mods...)
		newCmd = tea.Batch(newCmd, cmd)
	}
	return newModels, newCmd
}

func (h HandlerCollection) OnMessage(msg tea.Msg) {
	for _, handler := range h.handlers {
		if listener, ok := handler.(MessageListener); ok {
			listener.OnMessage(msg)
		}
	}
}

func (h HandlerCollection) Wait() {
	for _, handler := range h.handlers {
		if listener, ok := handler.(HandleWaiter); ok {
			listener.Wait()
		}
	}
}
