package parsers

import (
	"fmt"

	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/go-progress"

	"github.com/anchore/stereoscope/pkg/event"
	"github.com/anchore/stereoscope/pkg/image"
	"github.com/anchore/stereoscope/pkg/image/containerd"
	"github.com/anchore/stereoscope/pkg/image/docker"
)

type ErrBadPayload struct {
	Type  partybus.EventType
	Field string
	Value any
}

func (e *ErrBadPayload) Error() string {
	return fmt.Sprintf("event='%s' has bad event payload field='%v': '%+v'", string(e.Type), e.Field, e.Value)
}

func newPayloadErr(t partybus.EventType, field string, value any) error {
	return &ErrBadPayload{
		Type:  t,
		Field: field,
		Value: value,
	}
}

func checkEventType(actual, expected partybus.EventType) error {
	if actual != expected {
		return newPayloadErr(expected, "Type", actual)
	}
	return nil
}

func ParsePullDockerImage(e partybus.Event) (string, *docker.PullStatus, error) {
	if err := checkEventType(e.Type, event.PullDockerImage); err != nil {
		return "", nil, err
	}

	imgName, ok := e.Source.(string)
	if !ok {
		return "", nil, newPayloadErr(e.Type, "Source", e.Source)
	}

	pullStatus, ok := e.Value.(*docker.PullStatus)
	if !ok {
		return "", nil, newPayloadErr(e.Type, "Value", e.Value)
	}

	return imgName, pullStatus, nil
}

func ParsePullContainerdImage(e partybus.Event) (string, *containerd.PullStatus, error) {
	if err := checkEventType(e.Type, event.PullContainerdImage); err != nil {
		return "", nil, err
	}

	imgName, ok := e.Source.(string)
	if !ok {
		return "", nil, newPayloadErr(e.Type, "Source", e.Source)
	}

	pullStatus, ok := e.Value.(*containerd.PullStatus)
	if !ok {
		return "", nil, newPayloadErr(e.Type, "Value", e.Value)
	}

	return imgName, pullStatus, nil
}

func ParseFetchImage(e partybus.Event) (string, progress.StagedProgressable, error) {
	if err := checkEventType(e.Type, event.FetchImage); err != nil {
		return "", nil, err
	}

	imgName, ok := e.Source.(string)
	if !ok {
		return "", nil, newPayloadErr(e.Type, "Source", e.Source)
	}

	prog, ok := e.Value.(progress.StagedProgressable)
	if !ok {
		return "", nil, newPayloadErr(e.Type, "Value", e.Value)
	}

	return imgName, prog, nil
}

func ParseReadImage(e partybus.Event) (*image.Metadata, progress.Progressable, error) {
	if err := checkEventType(e.Type, event.ReadImage); err != nil {
		return nil, nil, err
	}

	imgMetadata, ok := e.Source.(image.Metadata)
	if !ok {
		return nil, nil, newPayloadErr(e.Type, "Source", e.Source)
	}

	prog, ok := e.Value.(progress.Progressable)
	if !ok {
		return nil, nil, newPayloadErr(e.Type, "Value", e.Value)
	}

	return &imgMetadata, prog, nil
}

func ParseReadLayer(e partybus.Event) (*image.LayerMetadata, progress.Monitorable, error) {
	if err := checkEventType(e.Type, event.ReadLayer); err != nil {
		return nil, nil, err
	}

	layerMetadata, ok := e.Source.(image.LayerMetadata)
	if !ok {
		return nil, nil, newPayloadErr(e.Type, "Source", e.Source)
	}

	prog, ok := e.Value.(progress.Monitorable)
	if !ok {
		return nil, nil, newPayloadErr(e.Type, "Value", e.Value)
	}

	return &layerMetadata, prog, nil
}
