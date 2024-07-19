// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package main is the entrypoint for the Zarf binary.
package main

import (
	"context"
	"embed"
	"os"
	"os/signal"
	"syscall"

	"github.com/zarf-dev/zarf/src/cmd"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/packager/lint"
)

//go:embed cosign.pub
var cosignPublicKey string

//go:embed zarf.schema.json
var zarfSchema embed.FS

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		first := true
		for {
			<-signalCh
			if first {
				first = false
				cancel()
				continue
			}
			os.Exit(1)
		}
	}()

	config.CosignPublicKey = cosignPublicKey
	lint.ZarfSchema = zarfSchema
	cmd.Execute(ctx)
}
