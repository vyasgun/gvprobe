package gvproxy

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shirou/gopsutil/process"
)

func Stop() error {
	pid, err := readPIDFromFile()
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, errInvalidPIDFile) {
			closeVfkitLocalListener()
			return fmt.Errorf("gvproxy is not running")
		}
		return err
	}
	if !runningGvproxyByPID(pid) {
		closeVfkitLocalListener()
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

	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()

	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-t.C:
			if !runningGvproxyByPID(pid) {
				closeVfkitLocalListener()
				return nil
			}
		case <-timeout:
			return fmt.Errorf("gvproxy did not stop")
		}
	}
}
