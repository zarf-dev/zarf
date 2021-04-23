package main

import (
	"embed"
	"shift/k3s"
	"shift/utils"

	log "github.com/sirupsen/logrus"
)

//go:embed assets
var assets embed.FS

func main() {

	runPreflightChecks()

	utils.WriteAssets(assets, "assets/k3s", "/usr/local/bin")
	utils.WriteAssets(assets, "assets/charts", "/var/lib/rancher/k3s/server/static/charts")
	utils.WriteAssets(assets, "assets/images", "/var/lib/rancher/k3s/agent/images")
	utils.WriteAssets(assets, "assets/manifests", "/var/lib/rancher/k3s/server/manifests")

	k3s.Install()
	// TODO: vip, chmod k3s bins, add k3s script exec steps
}

func runPreflightChecks() {
	if !utils.IsLinux() {
		log.Fatal("This program requires a Linux OS")
	}

	if !utils.IsAMD64() {
		log.Fatal("This program currently only runs on AMD64 architectures")
	}

	if !utils.IsUserRoot() {
		log.Fatal("You must run this program as root.")
	}

	if !utils.InvalidPath("/var/lib/rancher/k3s") {
		log.Fatal("")
	}
}
