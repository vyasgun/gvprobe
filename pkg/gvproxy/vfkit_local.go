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

	qemuLocalMu   sync.Mutex
	qemuLocalConn net.Conn
)

func closeLocalListeners() {
	closeVfkitLocalListener()
	closeBessConn()
	closeQemuConn()
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

func closeBessConn() {
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

// WithBessConn runs fn with a connected unixpacket socket to gvproxy's bess listener.
// Unlike vfkit (unixgram + WriteTo), SOCK_SEQPACKET requires connect before send/recv.
func WithBessConn(fn func(*net.UnixConn) error) error {
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

func closeQemuConn() {
	qemuLocalMu.Lock()
	defer qemuLocalMu.Unlock()
	if qemuLocalConn != nil {
		_ = qemuLocalConn.Close()
		qemuLocalConn = nil
	}
}

// WithQemuConn runs fn with a connected unix stream socket to gvproxy's qemu listener.
func WithQemuConn(fn func(net.Conn) error) error {
	qemuLocalMu.Lock()
	defer qemuLocalMu.Unlock()
	if qemuLocalConn == nil {
		c, err := net.Dial("unix", constants.QemuSocket)
		if err != nil {
			return fmt.Errorf("qemu connect %s (is gvproxy running with --backend qemu?): %w", constants.QemuSocket, err)
		}
		qemuLocalConn = c
	}
	return fn(qemuLocalConn)
}
