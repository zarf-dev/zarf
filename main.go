package main

import (
	"embed"

	"github.com/defenseunicorns/zarf/src/cmd"
	"github.com/defenseunicorns/zarf/src/config"
)

//go:embed all:build/ui/*
var assets embed.FS

//go:embed cosign.pub
var cosignPublicKey string

func main() {

	config.UIAssets = assets
	config.SGetPublicKey = cosignPublicKey
	cmd.Execute()
}
