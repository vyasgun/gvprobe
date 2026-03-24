package gvproxy

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/vyasgun/gvprobe/pkg/constants"
)

func Start() (int, error) {
	pid, err := readPIDFromFile()
	if err == nil {
		if runningGvproxyByPID(pid) {
			log.Printf("gvproxy is already running at pid %d", pid)
			return pid, nil
		}
		log.Printf("removing invalid pid file %s", constants.GvproxyPidFile)
		_ = os.Remove(constants.GvproxyPidFile)
	} else if errors.Is(err, errInvalidPIDFile) {
		log.Printf("removing invalid pid file %s", constants.GvproxyPidFile)
		_ = os.Remove(constants.GvproxyPidFile)
	}

	cmd := gvproxy.NewGvproxyCommand()
	cmd.PidFile = constants.GvproxyPidFile
	cmd.LogFile = constants.GvproxyLogFile
	runCmd := cmd.Cmd("gvproxy")
	runCmd.Env = os.Environ()
	if err := runCmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start gvproxy: %w", err)
	}
	child := runCmd.Process.Pid
	log.Printf("starting gvproxy...")
	time.Sleep(1 * time.Second)
	if runningGvproxyByPID(child) {
		return child, nil
	}
	return 0, fmt.Errorf("gvproxy did not start")
}
