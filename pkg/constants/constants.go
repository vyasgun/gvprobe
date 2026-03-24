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
)
