// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
// forked from https://www.socketloop.com/tutorials/golang-byte-format-example

// Package utils provides generic helper functions.
package utils

import (
	"context"
	"sync"
)

type ConcurrencyTools[P any, E any] struct {
	ProgressChan chan P
	ErrorChan    chan E
	context      context.Context
	Cancel       context.CancelFunc
	waitGroup    *sync.WaitGroup
	routineCount int
}

func NewConcurrencyTools[P any, E any](length int) *ConcurrencyTools[P, E] {
	ctx, cancel := context.WithCancel(context.Background())

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

func (ct *ConcurrencyTools[P, E]) IsDone() bool {
	ctx := ct.context
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func (ct *ConcurrencyTools[P, E]) WaitGroupDone() {
	ct.waitGroup.Done()
}

func (ct *ConcurrencyTools[P, E]) WaitWithProgress(onProgress func(P, int), onError func(E) error) error {
	for i := 0; i < ct.routineCount; i++ {
		select {
		case err := <-ct.ErrorChan:
			ct.Cancel()
			errResult := onError(err)
			ct.waitGroup.Done()
			return errResult
		case progress := <-ct.ProgressChan:
			onProgress(progress, i)
		}
	}
	ct.waitGroup.Wait()
	return nil
}
