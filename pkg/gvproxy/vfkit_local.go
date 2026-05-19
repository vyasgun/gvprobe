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

	bessLocalMu   sync.Mutex
	bessLocalConn *net.UnixConn
)

func closeLocalListeners() {
	closeVfkitLocalListener()
	closeBessLocalListener()
}

func closeVfkitLocalListener() {
	vfkitLocalMu.Lock()
	defer vfkitLocalMu.Unlock()
	if vfkitLocalConn != nil {
		_ = vfkitLocalConn.Close()
		vfkitLocalConn = nil
	}
	_ = os.Remove(constants.VfkitLocalSocket)
}

func closeBessLocalListener() {
	bessLocalMu.Lock()
	defer bessLocalMu.Unlock()
	if bessLocalConn != nil {
		_ = bessLocalConn.Close()
		bessLocalConn = nil
	}
	_ = os.Remove(constants.BessLocalSocket)
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

// WithBessLocal runs fn with a connected unixpacket socket to gvproxy's bess listener.
// Unlike vfkit (unixgram + WriteTo), SOCK_SEQPACKET requires connect before send/recv.
func WithBessLocal(fn func(*net.UnixConn) error) error {
	bessLocalMu.Lock()
	defer bessLocalMu.Unlock()
	if bessLocalConn == nil {
		_ = os.Remove(constants.BessLocalSocket)
		c, err := net.DialUnix("unixpacket",
			&net.UnixAddr{Name: constants.BessLocalSocket, Net: "unixpacket"},
			&net.UnixAddr{Name: constants.BessSocket, Net: "unixpacket"},
		)
		if err != nil {
			return fmt.Errorf("bess connect %s: %w", constants.BessSocket, err)
		}
		bessLocalConn = c
	}
	return fn(bessLocalConn)
}
