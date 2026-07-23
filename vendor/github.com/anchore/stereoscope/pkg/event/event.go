package event

import (
	"github.com/wagoodman/go-partybus"
)

const (
	PullDockerImage     partybus.EventType = "pull-docker-image-event"
	PullContainerdImage partybus.EventType = "pull-containerd-image-event"
	FetchImage          partybus.EventType = "fetch-image-event"
	ReadImage           partybus.EventType = "read-image-event"
	ReadLayer           partybus.EventType = "read-layer-event"
)
