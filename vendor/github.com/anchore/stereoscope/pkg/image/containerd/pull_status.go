package containerd

import (
	"context"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/wagoodman/go-progress"
)

// StatusInfoStatus describes status info for an upload or download.
type StatusInfoStatus string

const (
	StatusResolved    StatusInfoStatus = "resolved"
	StatusResolving   StatusInfoStatus = "resolving"
	StatusWaiting     StatusInfoStatus = "waiting"
	StatusCommitting  StatusInfoStatus = "committing"
	StatusDone        StatusInfoStatus = "done"
	StatusDownloading StatusInfoStatus = "downloading"
	StatusUploading   StatusInfoStatus = "uploading"
	StatusExists      StatusInfoStatus = "exists"
)

type LayerID string

type PullStatus struct {
	state    apiState
	layers   []LayerID
	progress map[LayerID]*progress.Manual
	lock     *sync.RWMutex
}

func newPullStatus(client *client.Client, ongoing *jobs) *PullStatus {
	return &PullStatus{
		state:    newAPIState(client, ongoing),
		progress: make(map[LayerID]*progress.Manual),
		lock:     &sync.RWMutex{},
	}
}

func (ps *PullStatus) Complete() bool {
	_, done := ps.state.current()
	return done
}

func (ps *PullStatus) Layers() []LayerID {
	ordered, _ := ps.state.current()

	var layers []LayerID
	for _, status := range ordered {
		layers = append(layers, LayerID(status.Ref))
	}

	return layers
}

func (ps *PullStatus) Current(layer LayerID) progress.Progressable {
	ps.state.lock.RLock()
	defer ps.state.lock.RUnlock()

	p := ps.progress[layer]
	if p == nil {
		return progress.NewManual(-1)
	}
	return p
}

func (s *apiState) current() ([]statusInfo, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return append([]statusInfo{}, s.ordered...), s.done
}

func (ps *PullStatus) start(ctx context.Context) *PullStatus {
	go func() {
		for !ps.state.done {
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				ps.update(ctx)
			}
		}
	}()
	return ps
}

func (ps *PullStatus) update(ctx context.Context) {
	// get the latest API state
	ps.state.update(ctx)

	// use the API state to update the progress that can drive callers (UIs)
	ordered, done := ps.state.current()
	ps.lock.Lock()
	defer ps.lock.Unlock()

	ps.layers = nil

	for _, status := range ordered {
		layer := LayerID(status.Ref)
		if status.Status == "" {
			continue
		}
		if _, ok := ps.progress[layer]; !ok {
			ps.progress[layer] = progress.NewManual(status.Total)
		} else {
			// based on the behavior of containerd, these values were found to drift
			// during initialization. Let's make certain we're using the latest values
			ps.progress[layer].SetTotal(status.Total)
		}
		ps.progress[layer].Set(status.Offset)
		if done {
			// TODO: is this right? or do we want to show intermediate failures at the spot they failed?
			ps.progress[layer].SetCompleted()
		}
		ps.layers = append(ps.layers, layer)
	}
}
