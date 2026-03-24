package gvproxy

import (
	"errors"
	"os"
)

type GvproxyStatus int

const (
	Running GvproxyStatus = iota
	Stopped
)

func (s GvproxyStatus) String() string {
	switch s {
	case Running:
		return "running"
	case Stopped:
		return "stopped"
	default:
		return "unknown"
	}
}

func Status() (GvproxyStatus, error) {
	pid, err := readPIDFromFile()
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, errInvalidPIDFile) {
			return Stopped, nil
		}
		return Stopped, err
	}
	if !runningGvproxyByPID(pid) {
		return Stopped, nil
	}
	return Running, nil
}
