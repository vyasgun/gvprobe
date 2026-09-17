package constants

import (
	"os"
	"path/filepath"
	"strings"
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
	QemuSocket       = filepath.Join(GvprobeConfigDir, "qemu.sock")
	QemuLocalSocket  = filepath.Join(GvprobeConfigDir, "qemu-local.sock")

	AttachPidFile = filepath.Join(GvprobeConfigDir, "attach.pid")
	AttachSocket  = filepath.Join(GvprobeConfigDir, "attach.sock")

	BackendFile = filepath.Join(GvprobeConfigDir, "backend")

	// GuestMAC matches gvproxy's default DHCP static lease for 192.168.127.2.
	GuestMAC = "5a:94:ef:e4:0c:ee"
	GuestIP  = "192.168.127.2"

	GatewayIP = "192.168.127.1"
	// GvproxySSHPort avoids clashing with Podman/other gvproxy defaults on 2222.
	GvproxySSHPort        = 12222
	GvproxyServicesListen = "tcp://0.0.0.0:5555"
	GvproxyURL            = "http://0.0.0.0:5555"
	LeasesURL             = GvproxyURL + "/services/dhcp/leases"

	DefaultBackend = "vfkit"
)

// WriteBackend persists the active backend to ~/.gvprobe/backend.
func WriteBackend(backend string) error {
	return os.WriteFile(BackendFile, []byte(backend), 0644)
}

// ReadBackend returns the saved backend or DefaultBackend if the file is missing.
func ReadBackend() string {
	b, err := os.ReadFile(BackendFile)
	if err != nil {
		return DefaultBackend
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return DefaultBackend
	}
	return s
}
