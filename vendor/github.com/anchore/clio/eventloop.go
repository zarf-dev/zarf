package clio

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/wagoodman/go-partybus"

	"github.com/anchore/go-logger"
)

// eventloop listens to worker errors (from execution path), worker events (from a partybus subscription), and
// signal interrupts. Is responsible for handling each event relative to a given UI to coordinate eventing until
// an eventual graceful exit.
//
//nolint:gocognit
func eventloop(ctx context.Context, log logger.Logger, subscription *partybus.Subscription, workerErrs <-chan error, ux UI) error {
	var events <-chan partybus.Event
	if subscription != nil {
		events = subscription.Events()
	} else {
		noEvents := make(chan partybus.Event)
		close(noEvents)
		events = noEvents
	}

	if ux != nil {
		err := ux.Setup(subscription)
		if err != nil {
			return fmt.Errorf("unable to setup UI: %w", err)
		}
	}

	var retErr []error
	var forceTeardown bool

	for workerErrs != nil || events != nil {
		select {
		case err, isOpen := <-workerErrs:
			if !isOpen {
				log.Trace("worker stopped")
				workerErrs = nil
				continue
			}
			if err != nil {
				// capture the error from the worker and unsubscribe to complete a graceful shutdown
				retErr = append(retErr, err)
				if subscription != nil {
					_ = subscription.Unsubscribe()
				}
				// the worker has exited, we may have been mid-handling events for the UI which should now be
				// ignored, in which case forcing a teardown of the UI regardless of the state is required.
				forceTeardown = true
			}
		case e, isOpen := <-events:
			if !isOpen {
				log.Trace("bus stopped")
				events = nil
				continue
			}

			if e.Type == ExitEventType {
				events = nil

				if e.Value == os.Interrupt {
					// on top of listening to signals from the OS, we also listen to interrupt events from the UI.
					// Why provide two ways to do the same thing? In an application that has a UI where the terminal
					// has been set to raw mode, ctrl-c will not register as an interrupt signal to the application.
					// Instead the UI will capture the ctrl-c and need to signal the event loop to exit gracefully.
					// Using the same signal channel for both OS signals and UI signals is not advisable and requires
					// injecting the channel into the UI (also not advisable). Providing a bus event for the UI to
					// use is a better solution here.

					log.Trace("signal interrupt")

					workerErrs = nil
					forceTeardown = true
				} else {
					log.Trace("signal exit")
				}

				if subscription != nil {
					_ = subscription.Unsubscribe()
				}
			}

			if ux == nil {
				continue
			}

			if err := ux.Handle(e); err != nil {
				if errors.Is(err, partybus.ErrUnsubscribe) {
					events = nil
				} else {
					retErr = append(retErr, err)
					// TODO: should we unsubscribe? should we try to halt execution? or continue?
				}
			}
		case <-ctx.Done():
			log.Trace("signal interrupt")

			// ignore further results from any event source and exit ASAP, but ensure that all cache is cleaned up.
			// we ignore further errors since cleaning up the tmp directories will affect running catalogers that are
			// reading/writing from/to their nested temp dirs. This is acceptable since we are bailing without result.

			// TODO: potential future improvement would be to pass context into workers with a cancel function that is
			// to the event loop. In this way we can have a more controlled shutdown even at the most nested levels
			// of processing.
			events = nil
			workerErrs = nil
			forceTeardown = true
		}
	}
	if ux != nil {
		if err := ux.Teardown(forceTeardown); err != nil {
			retErr = append(retErr, err)
		}
	}

	return errors.Join(retErr...)
}
