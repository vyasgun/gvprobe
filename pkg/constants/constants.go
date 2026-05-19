package constants

import (
	"os"
	"path/filepath"
)

var (
	HomeDir, _       = os.UserHomeDir()
	GvprobeConfigDir = filepath.Join(HomeDir, ".gvprobe")

	GvproxyPidFile = filepath.Join(GvprobeConfigDir, "gvproxy.pid")
	GvproxyLogFile = filepath.Join(GvprobeConfigDir, "gvproxy.log")

	VfkitSocket      = filepath.Join(GvprobeConfigDir, "vfkit.sock")
	VfkitLocalSocket = filepath.Join(GvprobeConfigDir, "vfkit-local.sock")
	BessSocket       = filepath.Join(GvprobeConfigDir, "bess.sock")
	BessLocalSocket  = filepath.Join(GvprobeConfigDir, "bess-local.sock")

	// GuestMAC matches gvproxy's default DHCP static lease for 192.168.127.2.
	GuestMAC = "5a:94:ef:e4:0c:ee"
	GuestIP  = "192.168.127.2"

	GatewayIP = "192.168.127.1"
	// GvproxySSHPort avoids clashing with Podman/other gvproxy defaults on 2222.
	GvproxySSHPort        = 12222
	GvproxyServicesListen = "tcp://0.0.0.0:5555"
	GvproxyURL            = "http://0.0.0.0:5555"
	LeasesURL             = GvproxyURL + "/services/dhcp/leases"
)
