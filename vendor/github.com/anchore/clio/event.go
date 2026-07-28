package clio

import (
	"os"

	"github.com/wagoodman/go-partybus"
)

const ExitEventType partybus.EventType = "clio-exit"

func ExitEvent(interrupt bool) partybus.Event {
	if interrupt {
		return partybus.Event{
			Type:  ExitEventType,
			Value: os.Interrupt,
		}
	}
	return partybus.Event{
		Type: ExitEventType,
	}
}
