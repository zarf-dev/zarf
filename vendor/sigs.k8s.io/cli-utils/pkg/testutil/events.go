// Copyright 2020 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"fmt"
	"sort"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type ExpEvent struct {
	EventType event.Type

	InitEvent        *ExpInitEvent
	ErrorEvent       *ExpErrorEvent
	ActionGroupEvent *ExpActionGroupEvent
	ApplyEvent       *ExpApplyEvent
	StatusEvent      *ExpStatusEvent
	PruneEvent       *ExpPruneEvent
	DeleteEvent      *ExpDeleteEvent
	WaitEvent        *ExpWaitEvent
	ValidationEvent  *ExpValidationEvent
}

type ExpInitEvent struct {
	// TODO: enable if we want to more thuroughly test InitEvents
	// ActionGroups []event.ActionGroup
}

type ExpErrorEvent struct {
	Err error
}

type ExpActionGroupEvent struct {
	GroupName string
	Action    event.ResourceAction
	Type      event.ActionGroupEventStatus
}

type ExpApplyEvent struct {
	GroupName  string
	Status     event.ApplyEventStatus
	Identifier object.ObjMetadata
	Error      error
}

type ExpStatusEvent struct {
	Status     status.Status
	Identifier object.ObjMetadata
	Error      error
}

type ExpPruneEvent struct {
	GroupName  string
	Status     event.PruneEventStatus
	Identifier object.ObjMetadata
	Error      error
}

type ExpDeleteEvent struct {
	GroupName  string
	Status     event.DeleteEventStatus
	Identifier object.ObjMetadata
	Error      error
}

type ExpWaitEvent struct {
	GroupName  string
	Status     event.WaitEventStatus
	Identifier object.ObjMetadata
}

type ExpValidationEvent struct {
	Identifiers object.ObjMetadataSet
	Error       error
}

func VerifyEvents(expEvents []ExpEvent, events []event.Event) error {
	if len(expEvents) == 0 && len(events) == 0 {
		return nil
	}
	expEventIndex := 0
	for i := range events {
		e := events[i]
		ee := expEvents[expEventIndex]
		if isMatch(ee, e) {
			expEventIndex++
			if expEventIndex >= len(expEvents) {
				return nil
			}
		}
	}
	return fmt.Errorf("event %s not found", expEvents[expEventIndex].EventType)
}

// nolint:gocyclo
// TODO(mortent): This function is pretty complex and with quite a bit of
// duplication. We should see if there is a better way to provide a flexible
// way to verify that we go the expected events.
func isMatch(ee ExpEvent, e event.Event) bool {
	if ee.EventType != e.Type {
		return false
	}

	// nolint:gocritic
	switch e.Type {
	case event.ErrorType:
		a := ee.ErrorEvent

		if a == nil {
			return true
		}

		b := e.ErrorEvent

		if a.Err != nil {
			if !cmp.Equal(a.Err, b.Err, cmpopts.EquateErrors()) {
				return false
			}
		}
		return true

	case event.ActionGroupType:
		agee := ee.ActionGroupEvent

		if agee == nil {
			return true
		}

		age := e.ActionGroupEvent

		if agee.GroupName != age.GroupName {
			return false
		}

		if agee.Action != age.Action {
			return false
		}

		if agee.Type != age.Status {
			return false
		}
		return true

	case event.ApplyType:
		aee := ee.ApplyEvent
		// If no more information is specified, we consider it a match.
		if aee == nil {
			return true
		}
		ae := e.ApplyEvent

		if aee.Identifier != object.NilObjMetadata {
			if aee.Identifier != ae.Identifier {
				return false
			}
		}

		if aee.GroupName != "" {
			if aee.GroupName != ae.GroupName {
				return false
			}
		}

		if aee.Status != ae.Status {
			return false
		}

		if aee.Error != nil {
			return ae.Error != nil
		}
		return ae.Error == nil

	case event.StatusType:
		see := ee.StatusEvent
		if see == nil {
			return true
		}
		se := e.StatusEvent

		if see.Identifier != se.Identifier {
			return false
		}

		if see.Status != se.PollResourceInfo.Status {
			return false
		}

		if see.Error != nil {
			return se.Error != nil
		}
		return se.Error == nil

	case event.PruneType:
		pee := ee.PruneEvent
		if pee == nil {
			return true
		}
		pe := e.PruneEvent

		if pee.Identifier != object.NilObjMetadata {
			if pee.Identifier != pe.Identifier {
				return false
			}
		}

		if pee.GroupName != "" {
			if pee.GroupName != pe.GroupName {
				return false
			}
		}

		if pee.Status != pe.Status {
			return false
		}

		if pee.Error != nil {
			return pe.Error != nil
		}
		return pe.Error == nil

	case event.DeleteType:
		dee := ee.DeleteEvent
		if dee == nil {
			return true
		}
		de := e.DeleteEvent

		if dee.Identifier != object.NilObjMetadata {
			if dee.Identifier != de.Identifier {
				return false
			}
		}

		if dee.GroupName != "" {
			if dee.GroupName != de.GroupName {
				return false
			}
		}

		if dee.Status != de.Status {
			return false
		}

		if dee.Error != nil {
			return de.Error != nil
		}
		return de.Error == nil

	case event.WaitType:
		wee := ee.WaitEvent
		if wee == nil {
			return true
		}
		we := e.WaitEvent

		if wee.Identifier != object.NilObjMetadata {
			if wee.Identifier != we.Identifier {
				return false
			}
		}

		if wee.GroupName != "" {
			if wee.GroupName != we.GroupName {
				return false
			}
		}

		if wee.Status != we.Status {
			return false
		}
		return true

	case event.ValidationType:
		vee := ee.ValidationEvent
		if vee == nil {
			return true
		}
		ve := e.ValidationEvent

		if vee.Identifiers != nil {
			if !vee.Identifiers.Equal(ve.Identifiers) {
				return false
			}
		}

		if vee.Error != nil {
			return ve.Error != nil
		}
		return ve.Error == nil

	default:
		return true
	}
}

func EventsToExpEvents(events []event.Event) []ExpEvent {
	result := make([]ExpEvent, 0, len(events))
	for _, event := range events {
		result = append(result, EventToExpEvent(event))
	}
	return result
}

func EventToExpEvent(e event.Event) ExpEvent {
	switch e.Type {
	case event.InitType:
		return ExpEvent{
			EventType: event.InitType,
			InitEvent: &ExpInitEvent{
				// TODO: enable if we want to more thuroughly test InitEvents
				// ActionGroups: e.InitEvent.ActionGroups,
			},
		}

	case event.ErrorType:
		return ExpEvent{
			EventType: event.ErrorType,
			ErrorEvent: &ExpErrorEvent{
				Err: e.ErrorEvent.Err,
			},
		}

	case event.ActionGroupType:
		return ExpEvent{
			EventType: event.ActionGroupType,
			ActionGroupEvent: &ExpActionGroupEvent{
				GroupName: e.ActionGroupEvent.GroupName,
				Action:    e.ActionGroupEvent.Action,
				Type:      e.ActionGroupEvent.Status,
			},
		}

	case event.ApplyType:
		return ExpEvent{
			EventType: event.ApplyType,
			ApplyEvent: &ExpApplyEvent{
				GroupName:  e.ApplyEvent.GroupName,
				Identifier: e.ApplyEvent.Identifier,
				Status:     e.ApplyEvent.Status,
				Error:      e.ApplyEvent.Error,
			},
		}

	case event.StatusType:
		return ExpEvent{
			EventType: event.StatusType,
			StatusEvent: &ExpStatusEvent{
				Identifier: e.StatusEvent.Identifier,
				Status:     e.StatusEvent.PollResourceInfo.Status,
				Error:      e.StatusEvent.Error,
			},
		}

	case event.PruneType:
		return ExpEvent{
			EventType: event.PruneType,
			PruneEvent: &ExpPruneEvent{
				GroupName:  e.PruneEvent.GroupName,
				Identifier: e.PruneEvent.Identifier,
				Status:     e.PruneEvent.Status,
				Error:      e.PruneEvent.Error,
			},
		}

	case event.DeleteType:
		return ExpEvent{
			EventType: event.DeleteType,
			DeleteEvent: &ExpDeleteEvent{
				GroupName:  e.DeleteEvent.GroupName,
				Identifier: e.DeleteEvent.Identifier,
				Status:     e.DeleteEvent.Status,
				Error:      e.DeleteEvent.Error,
			},
		}

	case event.WaitType:
		return ExpEvent{
			EventType: event.WaitType,
			WaitEvent: &ExpWaitEvent{
				GroupName:  e.WaitEvent.GroupName,
				Identifier: e.WaitEvent.Identifier,
				Status:     e.WaitEvent.Status,
			},
		}

	case event.ValidationType:
		return ExpEvent{
			EventType: event.ValidationType,
			ValidationEvent: &ExpValidationEvent{
				Identifiers: e.ValidationEvent.Identifiers,
				Error:       e.ValidationEvent.Error,
			},
		}
	}
	return ExpEvent{}
}

func RemoveEqualEvents(in []ExpEvent, expected ExpEvent) ([]ExpEvent, int) {
	matches := 0
	for i := 0; i < len(in); i++ {
		if cmp.Equal(in[i], expected, cmpopts.EquateErrors()) {
			// remove event at index i
			in = append(in[:i], in[i+1:]...)
			matches++
			i--
		}
	}
	return in, matches
}

// SortExpEvents sorts a list of ExpEvents so they can be compared for equality.
//
// This is a stable sort which only sorts nearly identical contiguous events by
// object identifier, to make the full list easier to validate.
//
// You may need to remove StatusEvents from the list before comparing, because
// these events are fully asynchronous and non-contiguous.
//
// Comparison Options:
// A) Expect(received).To(testutil.Equal(expected))
// B) testutil.assertEqual(t, expected, received)
func SortExpEvents(events []ExpEvent) {
	sort.SliceStable(events, GroupedEventsByID(events).Less)
}

// GroupedEventsByID implements sort.Interface for []ExpEvent based on
// the serialized ObjMetadata of Apply, Prune, and Delete events within the same
// task group.
// This makes testing events easier, because apply/prune/delete order is
// non-deterministic within each task group.
// This is only needed if you expect to have multiple apply/prune/delete events
// in the same task group.
type GroupedEventsByID []ExpEvent

func (ape GroupedEventsByID) Len() int      { return len(ape) }
func (ape GroupedEventsByID) Swap(i, j int) { ape[i], ape[j] = ape[j], ape[i] }
func (ape GroupedEventsByID) Less(i, j int) bool {
	if ape[i].EventType != ape[j].EventType {
		// don't change order if not the same type
		return i < j
	}
	switch ape[i].EventType {
	case event.ValidationType:
		// Validation events are predictable ordered by input object set order.
	case event.ApplyType:
		// Apply events are are predictably ordered by ordering.SortableMetas.
	case event.PruneType:
		// Prune events are predictably ordered in reverse apply order.
	case event.DeleteType:
		// Delete events are predictably ordered in reverse apply order.
	case event.WaitType:
		// Wait events are unpredictably ordered, because the status may
		// reconcile before or after the WaitTask starts, and status event
		// order after starting is dependent on remote controller behavior.
		// So here we sort status groups explicitly:
		// Pending > Skipped > Successful > Failed > Timeout.
		// Each status group is then sorted by Identifier:
		// Group > Kind > Namespace > Name.
		// Note that the Pending status is always optional.
		if ape[i].WaitEvent.GroupName == ape[j].WaitEvent.GroupName {
			if ape[i].WaitEvent.Status != ape[j].WaitEvent.Status {
				return lessWaitStatus(ape[i].WaitEvent.Status, ape[j].WaitEvent.Status)
			}
			return ape[i].WaitEvent.Identifier.String() < ape[j].WaitEvent.Identifier.String()
		}
	}
	return i < j
}

var waitStatusWeight = map[event.WaitEventStatus]int{
	event.ReconcilePending:    0,
	event.ReconcileSkipped:    1,
	event.ReconcileSuccessful: 2,
	event.ReconcileFailed:     3,
	event.ReconcileTimeout:    4,
}

func lessWaitStatus(x, y event.WaitEventStatus) bool {
	return waitStatusWeight[x] < waitStatusWeight[y]
}
