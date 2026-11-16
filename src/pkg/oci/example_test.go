// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci_test

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/zarf-dev/zarf/src/pkg/oci"
	"oras.land/oras-go/v2/registry/remote/retry"
)

func ExampleWithRetryPolicy() {
	policy := &retry.GenericPolicy{
		MaxRetry: 8,
		MinWait:  250 * time.Millisecond,
		MaxWait:  3 * time.Second,
		Backoff:  retry.DefaultBackoff,
		Retryable: func(response *http.Response, err error) (bool, error) {
			// Retry truncated responses, which ORAS's default predicate does not retry.
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return true, nil
			}
			return retry.DefaultPredicate(response, err)
		},
	}
	_, err := oci.NewOrasRemote("registry.example.com/packages/demo", oci.PlatformForArch("amd64"),
		oci.WithRetryPolicy(policy))
	if err != nil {
		panic(err)
	}
}
