package clio

import (
	"fmt"
	"sync"

	"github.com/wagoodman/go-partybus"
)

type UIConstructor func(Config) (*UICollection, error)

type UI interface {
	Setup(subscription partybus.Unsubscribable) error
	partybus.Handler
	Teardown(force bool) error
}

var _ UIConstructor = newUI

func newUI(Config) (*UICollection, error) {
	// gracefully degrade to no UI if no constructor is configured
	return NewUICollection(), nil
}

var _ UI = (*UICollection)(nil)

type UICollection struct {
	uis          []UI
	active       UI
	subscription partybus.Unsubscribable
	lock         *sync.Mutex
}

func NewUICollection(uis ...UI) *UICollection {
	return &UICollection{
		uis:  uis,
		lock: &sync.Mutex{},
	}
}

func (u *UICollection) Setup(subscription partybus.Unsubscribable) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.setup(subscription)
}

func (u *UICollection) setup(subscription partybus.Unsubscribable) error {
	u.subscription = subscription
	var setupErr error
	for _, ui := range u.uis {
		if err := ui.Setup(subscription); err != nil {
			setupErr = err
			continue
		}
		setupErr = nil

		u.active = ui
		break
	}
	return setupErr
}

func (u UICollection) Handle(event partybus.Event) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	if u.active == nil {
		return nil
	}
	return u.active.Handle(event)
}

func (u *UICollection) Teardown(force bool) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.teardown(force)
}

func (u *UICollection) teardown(force bool) error {
	if u.active == nil {
		return nil
	}
	err := u.active.Teardown(force)
	u.active = nil
	return err
}

func (u *UICollection) Replace(uis ...UI) error {
	u.lock.Lock()
	defer u.lock.Unlock()

	if u.subscription != nil {
		if err := u.teardown(false); err != nil {
			return fmt.Errorf("unable to teardown existing UI: %w", err)
		}
	}

	u.uis = uis

	if u.subscription != nil {
		err := u.setup(u.subscription)
		if err != nil {
			return fmt.Errorf("unable to setup UI replacement: %w", err)
		}
	}

	return nil
}
