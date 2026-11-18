// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci_test

import (
	"time"

	"github.com/oras-project/oras-go/v3/registry/remote/retry"
	"github.com/zarf-dev/zarf/src/pkg/oci"
)

func ExampleWithRetryPolicy() {
	policy := &retry.GenericPolicy{
		MaxRetry:  8,
		MinWait:   250 * time.Millisecond,
		MaxWait:   3 * time.Second,
		Backoff:   retry.DefaultBackoff,
		Retryable: retry.DefaultPredicate,
	}
	_, err := oci.NewOrasRemote("registry.example.com/packages/demo", oci.PlatformForArch("amd64"),
		oci.WithRetryPolicy(policy))
	if err != nil {
		panic(err)
	}
}
