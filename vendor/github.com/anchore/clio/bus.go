package clio

import (
	"github.com/wagoodman/go-partybus"
)

type BusConstructor func(Config) *partybus.Bus

var _ BusConstructor = newBus

func newBus(_ Config) *partybus.Bus {
	return partybus.NewBus()
}
