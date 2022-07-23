package main

// test
import (
	_ "embed"

	"github.com/defenseunicorns/zarf/src/cmd"
	"github.com/defenseunicorns/zarf/src/config"
)

//go:embed cosign.pub
var cosignPublicKey string

func main() {
	config.SGetPublicKey = cosignPublicKey
	cmd.Execute()
}
