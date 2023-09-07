// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
// forked from https://www.socketloop.com/tutorials/golang-byte-format-example

// Package utils provides generic helper functions.
package utils

import (
	"context"
	"sync"
)

// ConcurrencyTools is a struct that contains channels and a context for use in concurrent routines
type ConcurrencyTools[P any, E any] struct {
	ProgressChan chan P
	ErrorChan    chan E
	context      context.Context
	Cancel       context.CancelFunc
	waitGroup    *sync.WaitGroup
	routineCount int
}

// NewConcurrencyTools creates a new ConcurrencyTools struct
//
// Length is the number of iterations that will be performed concurrently
func NewConcurrencyTools[P any, E any](length int) *ConcurrencyTools[P, E] {
	ctx, cancel := context.WithCancel(context.TODO())

	progressChan := make(chan P, length)

	errorChan := make(chan E, length)

	waitGroup := sync.WaitGroup{}

	waitGroup.Add(length)

	concurrencyTools := ConcurrencyTools[P, E]{
		ProgressChan: progressChan,
		ErrorChan:    errorChan,
		context:      ctx,
		Cancel:       cancel,
		waitGroup:    &waitGroup,
		routineCount: length,
	}

	return &concurrencyTools
}

// IsDone returns true if the context is done.
func (ct *ConcurrencyTools[P, E]) IsDone() bool {
	ctx := ct.context
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// WaitGroupDone decrements the internal WaitGroup counter by one.
func (ct *ConcurrencyTools[P, E]) WaitGroupDone() {
	ct.waitGroup.Done()
}

// WaitWithProgress waits for all routines to finish
//
// onProgress is a callback function that is called when a routine sends a progress update
//
// onError is a callback function that is called when a routine sends an error
func (ct *ConcurrencyTools[P, E]) WaitWithProgress(onProgress func(P, int), onError func(E) error) error {
	for i := 0; i < ct.routineCount; i++ {
		select {
		case err := <-ct.ErrorChan:
			ct.Cancel()
			errResult := onError(err)
			return errResult
		case progress := <-ct.ProgressChan:
			onProgress(progress, i)
		}
	}
	ct.waitGroup.Wait()
	return nil
}

// WaitWithoutProgress waits for all routines to finish without a progress callback
//
// onError is a callback function that is called when a routine sends an error
func (ct *ConcurrencyTools[P, E]) WaitWithoutProgress(onError func(E) error) error {
	for i := 0; i < ct.routineCount; i++ {
		select {
		case err := <-ct.ErrorChan:
			ct.Cancel()
			errResult := onError(err)
			return errResult
		case  <-ct.ProgressChan:
		}
	}
	ct.waitGroup.Wait()
	return nil
}
