package main

import (
	"embed"

	"github.com/defenseunicorns/zarf/src/cmd"
	"github.com/defenseunicorns/zarf/src/config"
)

//go:embed frontend/build/*
var assets embed.FS

//go:embed cosign.pub
var cosignPublicKeyUI string

func main() {

	config.UIAssets = assets
	config.SGetPublicKey = cosignPublicKeyUI
	cmd.Execute()
}
