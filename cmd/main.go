package main

import (
	"github.com/docker/machine/libmachine/drivers/plugin"
	qemu "github.com/machine-drivers/docker-machine-driver-qemu"
)

func main() {
	plugin.RegisterDriver(qemu.NewDriver("default", "path"))
}
