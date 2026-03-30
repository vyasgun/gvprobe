package gvproxy

import (
	"errors"
	"fmt"
	"log"
	"os"

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
	cmd := NewDefaulCommand()
	pid, err = RunCommand(cmd)
	if err != nil {
		return 0, fmt.Errorf("failed to start gvproxy: %w", err)
	}
	return pid, nil
}
