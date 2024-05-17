// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package common handles command configuration across all commands
package common

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// SuppressGlobalInterrupt suppresses the global error on an interrupt
var SuppressGlobalInterrupt = false

// SetBaseDirectory sets the base directory. This is a directory with a zarf.yaml.
func SetBaseDirectory(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "."
}

// ExitOnInterrupt catches an interrupt and exits with fatal error
func ExitOnInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		if !SuppressGlobalInterrupt {
			message.Fatal(lang.ErrInterrupt, lang.ErrInterrupt.Error())
		}
	}()
}

// NewClusterOrDie creates a new Cluster instance and waits for the cluster to be ready or throws a fatal error.
func NewClusterOrDie(ctx context.Context) *cluster.Cluster {
	timeoutCtx, cancel := context.WithTimeout(ctx, cluster.DefaultTimeout)
	defer cancel()
	c, err := cluster.NewClusterWithWait(timeoutCtx)
	if err != nil {
		message.Fatalf(err, "Failed to connect to cluster")
	}
	return c
}
