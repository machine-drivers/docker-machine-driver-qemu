package main

import (
	"github.com/docker/machine/libmachine/drivers/plugin"
	"github.com/jigtools/docker-machine-driver-qemu"
)

func main() {
	plugin.RegisterDriver(qemu.NewDriver("default", "path"))
}
