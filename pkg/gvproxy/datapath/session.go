package datapath

import (
	"fmt"
	"net"
	"time"

	"github.com/vyasgun/gvprobe/pkg/constants"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
)

// Session sends and receives raw Ethernet frames on a gvproxy datapath.
type Session interface {
	SendFrame(frame []byte) error
	RecvFrame(timeout time.Duration) ([]byte, error)
}

// WithSession runs fn with an open datapath session for backend.
func WithSession(backend string, fn func(Session) error) error {
	if backend == "" {
		backend = "vfkit"
	}
	switch backend {
	case "vfkit":
		return withVfkit(fn)
	case "bess":
		return withBess(fn)
	default:
		return fmt.Errorf("backend %q is not supported yet (supported: %s)", backend, joinBackends())
	}
}

// unixConnSession sends bare L2 frames over a Unix socket (unixgram or unixpacket).
type unixConnSession struct {
	conn   *net.UnixConn
	remote *net.UnixAddr
}

func (s *unixConnSession) SendFrame(frame []byte) error {
	if s.remote != nil {
		_, err := s.conn.WriteTo(frame, s.remote)
		return err
	}
	_, err := s.conn.Write(frame)
	return err
}

func (s *unixConnSession) RecvFrame(timeout time.Duration) ([]byte, error) {
	_ = s.conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 65536)
	var n int
	var err error
	if s.remote != nil {
		n, _, err = s.conn.ReadFrom(buf)
	} else {
		n, err = s.conn.Read(buf)
	}
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func withVfkit(fn func(Session) error) error {
	remote := &net.UnixAddr{Name: constants.VfkitSocket, Net: "unixgram"}
	return gvproxy.WithVfkitLocal(func(c *net.UnixConn) error {
		return fn(&unixConnSession{conn: c, remote: remote})
	})
}

func withBess(fn func(Session) error) error {
	return gvproxy.WithBessLocal(func(c *net.UnixConn) error {
		return fn(&unixConnSession{conn: c})
	})
}

func joinBackends() string {
	s := SupportedBackends[0]
	for i := 1; i < len(SupportedBackends); i++ {
		s += ", " + SupportedBackends[i]
	}
	return s
}
