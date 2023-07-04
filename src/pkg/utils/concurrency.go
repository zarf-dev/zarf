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
	Context      context.Context
	Cancel       context.CancelFunc
	WaitGroup    *sync.WaitGroup
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
		Context:      ctx,
		Cancel:       cancel,
		WaitGroup:    &waitGroup,
	}

	return &concurrencyTools
}

func ContextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
