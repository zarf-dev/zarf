//go:build !slim

package main

import (
	"embed"

	"github.com/defenseunicorns/zarf/src/cmd"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/ui"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed cosign.pub
var cosignPublicKeyUI string

func main() {

	config.SGetPublicKey = cosignPublicKeyUI
	cmd.Execute()

	Launch()
}

func Launch() {

	// Create an instance of the app structure
	app := ui.NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:            "Zarf",
		Width:            1024,
		Height:           768,
		Assets:           assets,
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
