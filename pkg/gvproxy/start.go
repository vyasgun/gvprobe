package gvproxy

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/vyasgun/gvprobe/pkg/constants"
)

var ErrGvproxyAlreadyRunning = errors.New("gvproxy is already running")

// StartupTimeout bounds how long Start waits for gvproxy to prove it stayed
// up. gvproxy exits within milliseconds on fatal config errors (e.g. a busy
// port or a leftover socket), so this only delays genuine successes briefly.
const StartupTimeout = 2 * time.Second

func Start(backend string) (int, error) {
	pid, err := readPIDFromFile()
	if err == nil {
		if runningGvproxyByPID(pid) {
			return pid, ErrGvproxyAlreadyRunning
		}
		log.Printf("removing invalid pid file %s", constants.GvproxyPidFile)
		_ = os.Remove(constants.GvproxyPidFile)
	} else if errors.Is(err, errInvalidPIDFile) {
		log.Printf("removing invalid pid file %s", constants.GvproxyPidFile)
		_ = os.Remove(constants.GvproxyPidFile)
	}

	// A SIGKILLed gvproxy leaves its listen socket behind; gvproxy refuses
	// to start ("already exists") when it finds one. We just confirmed no
	// gvproxy is running, so the file is stale — remove it.
	if stale := backendListenSocket(backend); stale != "" {
		if _, err := os.Stat(stale); err == nil {
			log.Printf("removing stale gvproxy socket %s", stale)
			_ = os.Remove(stale)
		}
	}

	cmd := NewCommand(backend)
	pid, err = RunCommand(cmd)
	if err != nil {
		return 0, fmt.Errorf("failed to start gvproxy: %w", err)
	}
	if !WaitForRunning(pid, StartupTimeout) {
		return 0, fmt.Errorf("gvproxy exited during startup: %s", fatalLogLine())
	}
	return pid, nil
}

// backendListenSocket returns the socket gvproxy listens on for backend.
func backendListenSocket(backend string) string {
	switch backend {
	case "bess":
		return constants.BessSocket
	case "qemu":
		return constants.QemuSocket
	default:
		return constants.VfkitSocket
	}
}

// fatalLogLine extracts the last level=fatal line from gvproxy's log, if any.
func fatalLogLine() string {
	data, err := os.ReadFile(constants.GvproxyLogFile)
	if err != nil {
		return fmt.Sprintf("(no log at %s)", constants.GvproxyLogFile)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.Contains(line, "level=fatal") {
			// keep only the message part of the logfmt line
			if idx := strings.Index(line, "level=fatal msg="); idx >= 0 {
				return strings.Trim(strings.TrimPrefix(line[idx:], "level=fatal msg="), `"`)
			}
			return line
		}
	}
	return "see " + constants.GvproxyLogFile
}
