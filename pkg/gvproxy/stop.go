package gvproxy

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/vyasgun/gvprobe/pkg/constants"
)

func Stop() error {
	pid, err := readPIDFromFile()
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, errInvalidPIDFile) {
			return fmt.Errorf("gvproxy is not running")
		}
		return err
	}
	if !runningGvproxyByPID(pid) {
		return fmt.Errorf("gvproxy is not running")
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return err
	}
	if err := proc.Terminate(); err != nil {
		return err
	}
	log.Printf("stopping gvproxy...")
	time.Sleep(5 * time.Second)
	if _, err := os.Stat(constants.GvproxyPidFile); os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("gvproxy did not stop")
}
