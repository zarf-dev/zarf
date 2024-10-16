// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package healthchecks run kstatus style health checks on a list of objects
package healthchecks

import (
	"context"
	"errors"
	"fmt"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/aggregator"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// Run waits for a list of Zarf healthchecks to reach a ready state.
func Run(ctx context.Context, watcher watcher.StatusWatcher, healthChecks []v1alpha1.NamespacedObjectKindReference) error {
	objs := []object.ObjMetadata{}
	for _, hc := range healthChecks {
		gv, err := schema.ParseGroupVersion(hc.APIVersion)
		if err != nil {
			return err
		}
		obj := object.ObjMetadata{
			GroupKind: schema.GroupKind{
				Group: gv.Group,
				Kind:  hc.Kind,
			},
			Namespace: hc.Namespace,
			Name:      hc.Name,
		}
		objs = append(objs, obj)
	}
	err := WaitForReady(ctx, watcher, objs)
	if err != nil {
		return err
	}
	return nil
}

// WaitForReadyRuntime waits for all of the objects to reach a ready state.
func WaitForReadyRuntime(ctx context.Context, sw watcher.StatusWatcher, robjs []runtime.Object) error {
	objs := []object.ObjMetadata{}
	for _, robj := range robjs {
		obj, err := object.RuntimeToObjMeta(robj)
		if err != nil {
			return err
		}
		objs = append(objs, obj)
	}
	return WaitForReady(ctx, sw, objs)
}

// WaitForReady waits for all of the objects to reach a ready state.
func WaitForReady(ctx context.Context, sw watcher.StatusWatcher, objs []object.ObjMetadata) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	eventCh := sw.Watch(cancelCtx, objs, watcher.Options{})
	statusCollector := collector.NewResourceStatusCollector(objs)
	done := statusCollector.ListenWithObserver(eventCh, collector.ObserverFunc(
		func(statusCollector *collector.ResourceStatusCollector, _ event.Event) {
			rss := []*event.ResourceStatus{}
			for _, rs := range statusCollector.ResourceStatuses {
				if rs == nil {
					continue
				}
				rss = append(rss, rs)
			}
			desired := status.CurrentStatus
			if aggregator.AggregateStatus(rss, desired) == desired {
				cancel()
				return
			}
		}),
	)
	<-done

	if statusCollector.Error != nil {
		return statusCollector.Error
	}

	// Only check parent context error, otherwise we would error when desired status is achieved.
	if ctx.Err() != nil {
		errs := []error{}
		for _, id := range objs {
			rs := statusCollector.ResourceStatuses[id]
			switch rs.Status {
			case status.CurrentStatus:
				message.Debugf("%s: %s ready", rs.Identifier.Name, rs.Identifier.GroupKind.Kind)
			case status.NotFoundStatus:
				errs = append(errs, fmt.Errorf("%s: %s not found", rs.Identifier.Name, rs.Identifier.GroupKind.Kind))
			default:
				errs = append(errs, fmt.Errorf("%s: %s not ready", rs.Identifier.Name, rs.Identifier.GroupKind.Kind))
			}
		}
		errs = append(errs, ctx.Err())
		return errors.Join(errs...)
	}

	return nil
}

// ImmediateWatcher should only be used for testing and returns the set status immediately.
type ImmediateWatcher struct {
	status status.Status
}

// NewImmediateWatcher returns a ImmediateWatcher.
func NewImmediateWatcher(status status.Status) *ImmediateWatcher {
	return &ImmediateWatcher{
		status: status,
	}
}

// Watch watches the given objects and immediately returns the configured status.
func (w *ImmediateWatcher) Watch(_ context.Context, objs object.ObjMetadataSet, _ watcher.Options) <-chan event.Event {
	eventCh := make(chan event.Event, len(objs))
	for _, obj := range objs {
		eventCh <- event.Event{
			Type: event.ResourceUpdateEvent,
			Resource: &event.ResourceStatus{
				Identifier: obj,
				Status:     w.status,
			},
		}
	}
	close(eventCh)
	return eventCh
}
