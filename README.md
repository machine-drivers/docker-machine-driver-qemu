# [QEMU](http://www.qemu.org/) Driver Plugin for `docker-machine`

Please note that this is a fork of [CandySunPlus/docker-machine-driver-qemu](https://github.com/CandySunPlus/docker-machine-driver-qemu) which includes a commit for QEMU 6.x support.

This repo is in turn just a fork of the original repo [machine-drivers/docker-machine-driver-qemu](https://github.com/machine-drivers/docker-machine-driver-qemu) which seems unmaintained as the last release was made in 2017 and PRs like the one by CandySunPlus have not been merged.

This fork aims to add installation instructions for future reference.

## Docker Engine Version

Since this plugin uses the now deprecated [Boot2Docker(https://github.com/boot2docker/boot2docker) the version installed by default is the last released 
before deprecation of said project which is 19.03.12.

The project now recommends to instead install [Docker Desktop](https://www.docker.com/products/docker-desktop).

`docker version` output when following the [installation Instructions](#installation-instructions):

```
Server: Docker Engine - Community
 Engine:
  Version:          19.03.12
  API version:      1.40 (minimum version 1.12)
  Go version:       go1.13.10
  Git commit:       48a66213fe
  Built:            Mon Jun 22 15:49:35 2020
  OS/Arch:          linux/amd64
  Experimental:     false
 containerd:
  Version:          v1.2.13
  GitCommit:        7ad184331fa3e55e52b890ea95e65ba581ae3429
 runc:
  Version:          1.0.0-rc10
  GitCommit:        dc9208a3303feef5b3839f4323d9beb36df0a9dd
 docker-init:
  Version:          0.18.0
  GitCommit:        fec3683
```

## Installation Instructions

These installation instructions should work on Linux as well as on macOS.

Clone this repo to a location of choice and change into the cloned directory:

```
git clone https://github.com/totoroot/docker-machine-driver-qemu /tmp/docker-machine-driver-qemu && cd /tmp/docker-machine-driver-qemu
```

Make the binary of the plugin and copy it to a location in your PATH:

```sh
make default
cp bin/docker-machine-driver-qemu /usr/local/bin/docker-machine-driver-qemu
```

Depending on your user's permissions chose you might need to copy it with elevated permissions:

```sh
sudo install /tmp/docker-machine-driver-qemu /usr/local/bin/docker-machine-driver-qemu
```

Verify that binary is in your PATH:

```sh
which docker-machine-driver-qemu
```

Note that the available arguments for `docker-machine` using the QEMU driver are listed [below](#qemu-options).
 
Create a docker-machine VM named "qemu-default" with QEMU driver:

```sh
docker-machine create --driver=qemu qemu-default
```

Evaluate environment variables for VM:

```sh
eval $(docker-machine env qemu-default)
```

Verify `docker` works as it should:

```sh
docker version
```

## QEMU Options

Create machines locally using [QEMU](http://www.qemu.org/).
This driver requires QEMU to be installed on your host.

```sh
docker-machine create --driver=qemu qemu-default
```

Options:

 - `--qemu-boot2docker-url`: The URL of the boot2docker image. Defaults to the latest available version.
 - `--qemu-disk-size`: Size of disk for the host in MB. Default: `20000`
 - `--qemu-memory`: Size of memory for the host in MB. Default: `1024`
 - `--qemu-cpu-count`: Number of CPUs. Default: `1`
 - `--qemu-program` : Name of the qemu program to run. Default: `qemu-system-x86_64`
 - `--qemu-display` : Show the graphical display output to the user. Default: false
 - `--qemu-display-type` : Select type of display to use (sdl/vnc=localhost:0/etc)
 - `--qemu-nographic` : Use -nographic instead of -display none. Default: false
 - `--qemu-virtio-drives` : Use virtio for drives (cdrom and disk). Default: false
 - `--qemu-network`: Networking to be used: user, tap or bridge. Default: `user`
 - `--qemu-network-interface`: Name of the network interface to be used for networking. Default: `tap0`
 - `--qemu-network-address`: IP of the network address to be used for networking.
 - `--qemu-network-bridge`: Name of the network bridge to be used for networking. Default: `br0`

The `--qemu-boot2docker-url` flag takes a few different forms.  By
default, if no value is specified for this flag, Machine will check locally for
a boot2docker ISO.  If one is found, that will be used as the ISO for the
created machine.  If one is not found, the latest ISO release available on
[boot2docker/boot2docker](https://github.com/boot2docker/boot2docker) will be
downloaded and stored locally for future use.  Note that this means you must run
`docker-machine upgrade` deliberately on a machine if you wish to update the "cached"
boot2docker ISO.

This is the default behavior (when `--qemu-boot2docker-url=""`), but the
option also supports specifying ISOs by the `http://` and `file://` protocols.
`file://` will look at the path specified locally to locate the ISO: for
instance, you could specify `--qemu-boot2docker-url
file://$HOME/Downloads/rc.iso` to test out a release candidate ISO that you have
downloaded already.  You could also just get an ISO straight from the Internet
using the `http://` form.

Note that when using virtio the drives will be mounted as `/dev/vda` and `/dev/vdb`,
instead of the usual `/dev/cdrom` and `/dev/sda`, since they are using paravirtualization.

If using the real network (tap or bridge), note that it needs a DHCP server running.
The user network has it's own NAT network, which usually means it is running on 10.0.2.15
Ultimately this driver should be able to query for IP, but for now the workaround is
to use `--qemu-network-address` (and fixed addresses) until that feature is implemented.

Environment variables:

Here comes the list of the supported variables with the corresponding options. If both environment
variable and CLI option are provided the CLI option takes the precedence.

| Environment variable              | CLI option                        |
|-----------------------------------|-----------------------------------|
| `QEMU_BOOT2DOCKER_URL`            | `--qemu-boot2docker-url`          |
| `QEMU_VIRTIO_DRIVES`              | `--qemu-virtio-drives`            |
