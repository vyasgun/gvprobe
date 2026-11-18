package gvproxy

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/vyasgun/gvprobe/pkg/constants"
)

var errInvalidPIDFile = errors.New("invalid gvproxy pid file")

func readPIDFromFile() (int, error) {
	b, err := os.ReadFile(constants.GvproxyPidFile)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0, errInvalidPIDFile
	}
	return pid, nil
}

// WaitForRunning polls after exec until the deadline; gvproxy can exit
// quickly on bind errors.
func WaitForRunning(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if runningGvproxyByPID(pid) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return runningGvproxyByPID(pid)
}

// runningGvproxyByPID reports whether pid refers to a live process named "gvproxy".
func runningGvproxyByPID(pid int) bool {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return false
	}
	ok, err := proc.IsRunning()
	if err != nil || !ok {
		return false
	}
	name, err := proc.Name()
	return err == nil && name == "gvproxy"
}
