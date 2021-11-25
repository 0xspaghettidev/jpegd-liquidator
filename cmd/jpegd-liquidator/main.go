package main

import (
	"flag"

	"github.com/alecthomas/kong"
	"github.com/golang/glog"
)

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	//assume the config file is in the same directory as the binary
	//configuration parameters can also be passed via CLI
	options := append([]kong.Option{kong.Configuration(kong.JSON, "./config.json")}, GetMappers()...)
	ctx := kong.Parse(&Commands, options...)
	err := ctx.Run()

	if err != nil {
		glog.Fatal(err.Error())
	}
}
