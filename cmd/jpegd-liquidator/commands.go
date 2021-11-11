package main

import (
	"github.com/0xspaghettidev/jpegd-liquidator/keystore"
	"github.com/0xspaghettidev/jpegd-liquidator/liquidator"
)

var Commands struct {
	Keystore   keystore.Cmd   `cmd help:"Creates a password protected keystore"`
	Liquidator liquidator.Cmd `cmd help:"Starts the liquidator bot"`
}
