// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package common handles command configuration across all commands
package common

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/defenseunicorns/zarf/src/config/lang"
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
