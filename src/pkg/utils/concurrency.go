// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
// forked from https://www.socketloop.com/tutorials/golang-byte-format-example

// Package utils provides generic helper functions.
package utils

import (
	"context"
	"sync"
)

// ConcurrencyTools is a struct that facilitates easier concurrency by providing a context, cancel function, wait group, progress channel, and error channel that is compatible with the WaitForConcurrencyTools function
type ConcurrencyTools[P any, E any] struct {
	ProgressChan chan P
	ErrorChan    chan E
	Context      context.Context
	Cancel       context.CancelFunc
	WaitGroup    *sync.WaitGroup
	RoutineCount int
}

// NewConcurrencyTools returns a ConcurrencyTools struct that has the given length set for concurrency iterations
func NewConcurrencyTools[P any, E any](length int) *ConcurrencyTools[P, E] {
	ctx, cancel := context.WithCancel(context.Background())

	progressChan := make(chan P, length)

	errorChan := make(chan E, length)

	waitGroup := sync.WaitGroup{}

	waitGroup.Add(length)

	concurrencyTools := ConcurrencyTools[P, E]{
		ProgressChan: progressChan,
		ErrorChan:    errorChan,
		Context:      ctx,
		Cancel:       cancel,
		WaitGroup:    &waitGroup,
		RoutineCount: length,
	}

	return &concurrencyTools
}

// ContextDone returns true if the context has been marked as done
func ContextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// ReturnError returns the error passed in
func ReturnError(err error) error {
	return err
}

// WaitForConcurrencyTools waits for the concurrencyTools passed in to finish or returns the first error it encounters, it calls the errorFunc if an error is encountered and the progressFunc if a progress update is received
func WaitForConcurrencyTools[P any, E any, PF func(P, int), EF func(E) error](concurrencyTools *ConcurrencyTools[P, E], progressFunc PF, errorFunc EF) error {
	for i := 0; i < concurrencyTools.RoutineCount; i++ {
		select {
		case err := <-concurrencyTools.ErrorChan:
			concurrencyTools.Cancel()
			errResult := errorFunc(err)
			return errResult
		case progress := <-concurrencyTools.ProgressChan:
			progressFunc(progress, i)
		}
	}
	concurrencyTools.WaitGroup.Wait()
	return nil
}
