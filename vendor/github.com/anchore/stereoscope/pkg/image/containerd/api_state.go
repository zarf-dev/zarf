package containerd

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/errdefs"
	"github.com/scylladb/go-set/strset"

	"github.com/anchore/stereoscope/internal/log"
)

// apiState represents anything needed to be persisted from the containerd API for pull status.
type apiState struct {
	client    *client.Client
	startedAt time.Time
	ongoing   *jobs
	store     content.Store
	statuses  map[string]statusInfo
	ordered   []statusInfo
	done      bool
	lock      *sync.RWMutex
}

// statusInfo holds the status info for an upload or download
type statusInfo struct {
	Ref       string
	Status    StatusInfoStatus
	Offset    int64
	Total     int64
	StartedAt time.Time
	UpdatedAt time.Time
}

func newAPIState(client *client.Client, ongoing *jobs) apiState {
	return apiState{
		lock:      &sync.RWMutex{},
		client:    client,
		startedAt: time.Now(),
		ongoing:   ongoing,
		store:     client.ContentStore(),
	}
}

func (s *apiState) update(ctx context.Context) {
	ordered := s.fetchCurrentState(ctx)

	s.lock.Lock()
	s.ordered = ordered
	s.lock.Unlock()

	seenStatus := strset.New()
	for _, status := range ordered {
		seenStatus.Add(string(status.Status))
	}

	// nice for debugging
	// l := seenStatus.List()
	// sort.Strings(l)
	// log.Tracef("containerd pull statuses: %v", l)

	if seenStatus.Size() == 0 {
		return
	}

	// remove known completed statuses...
	// if there are no more statuses, then we are done
	seenStatus.Remove(string(StatusDone), string(StatusExists), string(StatusResolved))

	if seenStatus.Size() != 0 {
		return
	}

	log.Trace("no active containerd object downloads")
	// check to see if the image exists in the store... if not, this is a race condition on the bus
	img, err := s.client.GetImage(ctx, s.ongoing.name)
	if err != nil || img == nil {
		log.Trace("unable to get containerd image status: ", err.Error())
		// probably not done yet... keep waiting
		return
	}
	log.Tracef("containerd image downloaded digest=%q", img.Metadata().Target.Digest)
	ordered = s.fetchCurrentState(ctx)

	s.lock.Lock()
	s.ordered = ordered
	s.done = true // allow ui to update once more
	s.lock.Unlock()
}

func (s *apiState) fetchCurrentState(ctx context.Context) []statusInfo {
	// note: this was HEAVILY derived from  https://github.com/containerd/containerd/blob/v1.7.0/cmd/ctr/commands/content/content.go
	// and is Apache 2.0 licensed

	s.statuses = map[string]statusInfo{}

	resolved := StatusResolved
	if !s.ongoing.IsResolved() {
		resolved = StatusResolving
	}
	s.statuses[s.ongoing.name] = statusInfo{
		Ref:    s.ongoing.name,
		Status: resolved,
	}
	keys := []string{s.ongoing.name}

	activeSeen := map[string]struct{}{}
	if !s.done {
		active, err := s.store.ListStatuses(ctx, "")
		if err != nil {
			// TODO: log?
			return nil
		}
		// update status of active entries!
		for _, active := range active {
			s.statuses[active.Ref] = statusInfo{
				Ref:       active.Ref,
				Status:    StatusDownloading,
				Offset:    active.Offset,
				Total:     active.Total,
				StartedAt: active.StartedAt,
				UpdatedAt: active.UpdatedAt,
			}
			activeSeen[active.Ref] = struct{}{}
		}
	}

	// now, update the items in jobs that are not in active
	for _, j := range s.ongoing.Jobs() {
		key := remotes.MakeRefKey(ctx, j)
		keys = append(keys, key)
		if _, ok := activeSeen[key]; ok {
			continue
		}

		status, ok := s.statuses[key]
		if !s.done && (!ok || status.Status == StatusDownloading) {
			info, err := s.store.Info(ctx, j.Digest)
			if err != nil { //nolint:gocritic
				if !errdefs.IsNotFound(err) {
					// TODO: log?
					return nil
				} else { //nolint:revive
					s.statuses[key] = statusInfo{
						Ref:    key,
						Status: StatusWaiting,
					}
				}
			} else if info.CreatedAt.After(s.startedAt) {
				s.statuses[key] = statusInfo{
					Ref:       key,
					Status:    StatusDone,
					Offset:    info.Size,
					Total:     info.Size,
					UpdatedAt: info.CreatedAt,
				}
			} else {
				s.statuses[key] = statusInfo{
					Ref:    key,
					Status: StatusExists,
				}
			}
		} else if s.done {
			if ok {
				if status.Status != StatusDone && status.Status != StatusExists {
					status.Status = StatusDone
					s.statuses[key] = status
				}
			} else {
				s.statuses[key] = statusInfo{
					Ref:    key,
					Status: StatusDone,
				}
			}
		}
	}

	var ordered []statusInfo
	for _, key := range keys {
		if !strings.Contains(key, "layer") {
			continue
		}
		ordered = append(ordered, s.statuses[key])
	}

	return ordered
}
