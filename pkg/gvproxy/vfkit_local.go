package gvproxy

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/vyasgun/gvprobe/pkg/constants"
)

var (
	vfkitLocalMu   sync.Mutex
	vfkitLocalConn *net.UnixConn
)

func closeVfkitLocalListener() {
	vfkitLocalMu.Lock()
	defer vfkitLocalMu.Unlock()
	if vfkitLocalConn != nil {
		_ = vfkitLocalConn.Close()
		vfkitLocalConn = nil
	}
	_ = os.Remove(constants.VfkitLocalSocket)
}

// WithVfkitLocal runs fn with the vfkit client unixgram socket.
// Each gvprobe process lazily binds vfkit-local.sock on first use (start/trace are separate processes).
func WithVfkitLocal(fn func(*net.UnixConn) error) error {
	vfkitLocalMu.Lock()
	defer vfkitLocalMu.Unlock()
	if vfkitLocalConn == nil {
		_ = os.Remove(constants.VfkitLocalSocket)
		c, err := net.ListenUnixgram("unixgram", &net.UnixAddr{
			Name: constants.VfkitLocalSocket,
			Net:  "unixgram",
		})
		if err != nil {
			return fmt.Errorf("vfkit local listener: %w", err)
		}
		vfkitLocalConn = c
	}
	return fn(vfkitLocalConn)
}
