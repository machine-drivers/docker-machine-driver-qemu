package qemu

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnflag"
	"github.com/docker/machine/libmachine/mcnutils"
	"github.com/docker/machine/libmachine/ssh"
	"github.com/docker/machine/libmachine/state"
)

const (
	isoFilename        = "boot2docker.iso"
	privateNetworkName = "docker-machines"

	defaultSSHUser = "docker"
)

type Driver struct {
	*drivers.BaseDriver

	Memory           int
	DiskSize         int
	CPU              int
	Network          string
	PrivateNetwork   string
	ISO              string
	Boot2DockerURL   string
	CaCertPath       string
	PrivateKeyPath   string
	DiskPath         string
	CacheMode        string
	IOMode           string
	connectionString string
	//	conn             *libvirt.Connect
	//	VM               *libvirt.Domain
	vmLoaded bool
}

func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		mcnflag.IntFlag{
			Name:  "qemu-memory",
			Usage: "Size of memory for host in MB",
			Value: 1024,
		},
		mcnflag.IntFlag{
			Name:  "qemu-disk-size",
			Usage: "Size of disk for host in MB",
			Value: 20000,
		},
		mcnflag.IntFlag{
			Name:  "qemu-cpu-count",
			Usage: "Number of CPUs",
			Value: 1,
		},
		// TODO - support for multiple networks
		mcnflag.StringFlag{
			Name:  "qemu-network",
			Usage: "Name of network to connect to",
			Value: "default",
		},
		mcnflag.StringFlag{
			EnvVar: "KVM_BOOT2DOCKER_URL",
			Name:   "qemu-boot2docker-url",
			Usage:  "The URL of the boot2docker image. Defaults to the latest available version",
			Value:  "",
		},
		mcnflag.StringFlag{
			Name:  "qemu-cache-mode",
			Usage: "Disk cache mode: default, none, writethrough, writeback, directsync, or unsafe",
			Value: "default",
		},
		mcnflag.StringFlag{
			Name:  "qemu-io-mode",
			Usage: "Disk IO mode: threads, native",
			Value: "threads",
		},
		mcnflag.StringFlag{
			EnvVar: "KVM_SSH_USER",
			Name:   "qemu-ssh-user",
			Usage:  "SSH username",
			Value:  defaultSSHUser,
		},
		/* Not yet implemented
		mcnflag.Flag{
			Name:  "qemu-no-share",
			Usage: "Disable the mount of your home directory",
		},
		*/
	}
}

func (d *Driver) GetMachineName() string {
	return d.MachineName
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

func (d *Driver) GetSSHKeyPath() string {
	return d.ResolveStorePath("id_rsa")
}

func (d *Driver) GetSSHPort() (int, error) {
	if d.SSHPort == 0 {
		d.SSHPort = 22
	}

	return d.SSHPort, nil
}

func (d *Driver) GetSSHUsername() string {
	if d.SSHUser == "" {
		d.SSHUser = "docker"
	}

	return d.SSHUser
}

func (d *Driver) DriverName() string {
	return "qemu"
}

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	log.Debugf("SetConfigFromFlags called")
	d.Memory = flags.Int("qemu-memory")
	d.DiskSize = flags.Int("qemu-disk-size")
	d.CPU = flags.Int("qemu-cpu-count")
	d.Network = flags.String("qemu-network")
	d.Boot2DockerURL = flags.String("qemu-boot2docker-url")
	d.CacheMode = flags.String("qemu-cache-mode")
	d.IOMode = flags.String("qemu-io-mode")

	d.SwarmMaster = flags.Bool("swarm-master")
	d.SwarmHost = flags.String("swarm-host")
	d.SwarmDiscovery = flags.String("swarm-discovery")
	d.ISO = d.ResolveStorePath(isoFilename)
	d.SSHUser = flags.String("qemu-ssh-user")
	d.SSHPort = 22
	d.DiskPath = d.ResolveStorePath(fmt.Sprintf("%s.img", d.MachineName))
	return nil
}

func (d *Driver) GetURL() (string, error) {
	log.Debugf("GetURL called")
	ip, err := d.GetIP()
	if err != nil {
		log.Warnf("Failed to get IP: %s", err)
		return "", err
	}
	if ip == "" {
		return "", nil
	}
	return fmt.Sprintf("tcp://%s:2376", ip), nil // TODO - don't hardcode the port!
}

func NewDriver(hostName, storePath string) drivers.Driver {
	return &Driver{
		PrivateNetwork: privateNetworkName,
		BaseDriver: &drivers.BaseDriver{
			SSHUser:     defaultSSHUser,
			MachineName: hostName,
			StorePath:   storePath,
		},
	}
}

func (d *Driver) GetIP() (string, error) {
	return "", nil
}

func (d *Driver) GetState() (state.State, error) {
	return state.None, nil
}

func (d *Driver) PreCreateCheck() error {
	return nil
}

func (d *Driver) Create() error {
	var err error
	d.SSHPort, err = getAvailableTCPPort()
	if err != nil {
		return err
	}
	b2dutils := mcnutils.NewB2dUtils(d.StorePath)
	if err := b2dutils.CopyIsoToMachineDir(d.Boot2DockerURL, d.MachineName); err != nil {
		return err
	}

	log.Infof("Creating SSH key...")

	if err := ssh.GenerateSSHKey(d.sshKeyPath()); err != nil {
		return err
	}

	log.Infof("Creating Disk image...")

	if err := d.generateDiskImage(d.DiskSize); err != nil {
		return err
	}
	return nil
}

func getAvailableTCPPort() (int, error) {
	// FIXME: this has a race condition between finding an available port and
	// virtualbox using that port. Perhaps we should randomly pick an unused
	// port in a range not used by kernel for assigning ports
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	addr := ln.Addr().String()
	addrParts := strings.SplitN(addr, ":", 2)
	return strconv.Atoi(addrParts[1])
}

func (d *Driver) Start() error {
	// fmt.Printf("Init qemu %s\n", i.VM)

	// qemu-system-x86_64 -net nic -net user -m 2048M -boot d -cdrom boot2docker.iso persist.raw

	// qemu-system-x86_64 -enable-kvm -m 4096 -cdrom boot2docker.iso -boot order=d -net nic,vlan=0 -net user,vlan=0 -serial stdio

	// http://en.wikibooks.org/wiki/QEMU/Networking <<<< and samba sharing directly..

	startCmd := []string{
		// TODO add as param		"-curses",
		// "-enable-kvm", // need to test to see if its available
		"-netdev", "user,id=network0",
		"-device", "e1000,netdev=network0",
		"-redir", fmt.Sprintf("tcp:%d::22", d.SSHPort),
		"-m", fmt.Sprintf("%d", d.Memory),
		"-boot", "d",
		"-cdrom", "./boot2docker.iso",
		d.diskPath(),
	}

	if stdout, stderr, err := cmdOutErr("qemu-system-x86_64", startCmd...); err != nil {
		fmt.Printf("OUTPUT: %s\n", stdout)
		fmt.Printf("ERROR: %s\n", stderr)
		//	if err := cmdStart("qemu-system-x86_64", startCmd...); err != nil {
		return err
	}
	log.Infof("Waiting for VM to start (ssh -p %d docker@localhost)...", d.SSHPort)

	return nil
	//return ssh.WaitForTCP(fmt.Sprintf("localhost:%d", d.SSHPort))
}

func cmdOutErr(cmdStr string, args ...string) (string, string, error) {
	cmd := exec.Command(cmdStr, args...)
	log.Debugf("executing: %v %v", cmdStr, strings.Join(args, " "))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	stderrStr := stderr.String()
	log.Debugf("STDOUT: %v", stdout.String())
	log.Debugf("STDERR: %v", stderrStr)
	if err != nil {
		if ee, ok := err.(*exec.Error); ok && ee == exec.ErrNotFound {
			err = fmt.Errorf("mystery error: %s", ee)
		}
	} else {
		// VBoxManage will sometimes not set the return code, but has a fatal error
		// such as VBoxManage.exe: error: VT-x is not available. (VERR_VMX_NO_VMX)
		if strings.Contains(stderrStr, "error:") {
			err = fmt.Errorf("%v %v failed: %v", cmdStr, strings.Join(args, " "), stderrStr)
		}
	}
	return stdout.String(), stderrStr, err
}

func cmdStart(cmdStr string, args ...string) error {
	cmd := exec.Command(cmdStr, args...)
	log.Debugf("executing: %v %v", cmdStr, strings.Join(args, " "))
	return cmd.Start()
}

func (d *Driver) Stop() error {
	return fmt.Errorf("hosts without a driver cannot be stopped")
}

func (d *Driver) Remove() error {
	return nil
}

func (d *Driver) Restart() error {
	return fmt.Errorf("hosts without a driver cannot be restarted")
}

func (d *Driver) Kill() error {
	return fmt.Errorf("hosts without a driver cannot be killed")
}

func (d *Driver) StartDocker() error {
	return fmt.Errorf("hosts without a driver cannot start docker")
}

func (d *Driver) StopDocker() error {
	return fmt.Errorf("hosts without a driver cannot stop docker")
}

func (d *Driver) GetDockerConfigDir() string {
	return ""
}

func (d *Driver) Upgrade() error {
	return fmt.Errorf("hosts without a driver cannot be upgraded")
}

//func (d *Driver) GetSSHCommand(args ...string) (*exec.Cmd, error) {
//	return ssh.GetSSHCommand("localhost", d.SSHPort, "docker", d.sshKeyPath(), args...), nil
//}

func (d *Driver) sshKeyPath() string {
	return filepath.Join(d.StorePath, "id_rsa")
}

func (d *Driver) publicSSHKeyPath() string {
	return d.sshKeyPath() + ".pub"
}

func (d *Driver) diskPath() string {
	return filepath.Join(d.StorePath, "disk.qcow2")
}

// Make a boot2docker VM disk image.
func (d *Driver) generateDiskImage(size int) error {
	log.Debugf("Creating %d MB hard disk image...", size)

	magicString := "boot2docker, please format-me"

	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	// magicString first so the automount script knows to format the disk
	file := &tar.Header{Name: magicString, Size: int64(len(magicString))}
	if err := tw.WriteHeader(file); err != nil {
		return err
	}
	log.Infof("1")
	if _, err := tw.Write([]byte(magicString)); err != nil {
		return err
	}
	// .ssh/key.pub => authorized_keys
	log.Infof("2")
	file = &tar.Header{Name: ".ssh", Typeflag: tar.TypeDir, Mode: 0700}
	if err := tw.WriteHeader(file); err != nil {
		return err
	}
	log.Infof("3")
	pubKey, err := ioutil.ReadFile(d.publicSSHKeyPath())
	if err != nil {
		return err
	}
	log.Infof("4")
	file = &tar.Header{Name: ".ssh/authorized_keys", Size: int64(len(pubKey)), Mode: 0644}
	if err := tw.WriteHeader(file); err != nil {
		return err
	}
	log.Infof("5")
	if _, err := tw.Write([]byte(pubKey)); err != nil {
		return err
	}
	log.Infof("6")
	file = &tar.Header{Name: ".ssh/authorized_keys2", Size: int64(len(pubKey)), Mode: 0644}
	if err := tw.WriteHeader(file); err != nil {
		return err
	}
	log.Infof("7")
	if _, err := tw.Write([]byte(pubKey)); err != nil {
		return err
	}
	log.Infof("8")
	if err := tw.Close(); err != nil {
		return err
	}
	log.Infof("9")
	rawFile := fmt.Sprintf("%s.raw", d.diskPath())
	if err := ioutil.WriteFile(rawFile, buf.Bytes(), 0644); err != nil {
		return nil
	}
	log.Infof("10")
	if stdout, stderr, err := cmdOutErr("qemu-img", "convert", "-f", "raw", "-O", "qcow2", rawFile, d.diskPath()); err != nil {
		fmt.Printf("OUTPUT: %s\n", stdout)
		fmt.Printf("ERROR: %s\n", stderr)
		return err
	}
	log.Infof("11")
	if stdout, stderr, err := cmdOutErr("qemu-img", "resize", d.diskPath(), fmt.Sprintf("+%dMB", size)); err != nil {
		fmt.Printf("OUTPUT: %s\n", stdout)
		fmt.Printf("ERROR: %s\n", stderr)
		return err
	}
	log.Debugf("DONE writing to %s and %s", rawFile, d.diskPath())

	return nil
}
