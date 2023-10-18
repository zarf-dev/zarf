// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package main is the entrypoint for the Zarf binary.
package main

import (
	_ "embed"

	"github.com/defenseunicorns/zarf/src/cmd"
	"github.com/defenseunicorns/zarf/src/config"
)

//go:embed cosign.pub
var cosignPublicKey string

func main() {
	config.CosignPublicKey = cosignPublicKey
	cmd.Execute()
}
