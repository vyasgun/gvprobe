package datapath

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/vyasgun/gvprobe/pkg/constants"
)

// WithSessionOrAttach routes through the attach proxy when it's running,
// otherwise falls back to a direct datapath session.
func WithSessionOrAttach(backend string, fn func(Session) error) error {
	if isAttachAvailable() {
		conn, err := net.Dial("unix", constants.AttachSocket)
		if err == nil {
			defer func() { _ = conn.Close() }()
			return fn(&attachSession{conn: conn})
		}
		// dial failed — fall through to direct
	}
	return WithSession(backend, fn)
}

// isAttachAvailable reports whether attach.sock exists (proxy is likely listening).
func isAttachAvailable() bool {
	_, err := os.Stat(constants.AttachSocket)
	return err == nil
}

// attachSession implements Session by proxying frames through attach.sock.
// SendFrame buffers the frame; RecvFrame sends it + timeout as one RPC.
type attachSession struct {
	conn    net.Conn
	pending []byte
}

func (s *attachSession) SendFrame(frame []byte) error {
	s.pending = make([]byte, len(frame))
	copy(s.pending, frame)
	return nil
}

func (s *attachSession) RecvFrame(timeout time.Duration) ([]byte, error) {
	if s.pending == nil {
		return nil, fmt.Errorf("attach proxy: SendFrame not called before RecvFrame")
	}
	frame := s.pending
	s.pending = nil

	// Write request: [4B timeout_ms][4B frame_len][frame]
	timeoutMs := uint32(timeout.Milliseconds())
	var hdr [8]byte
	binary.BigEndian.PutUint32(hdr[:4], timeoutMs)
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(frame)))
	if _, err := s.conn.Write(hdr[:]); err != nil {
		return nil, fmt.Errorf("attach proxy send: %w", err)
	}
	if _, err := s.conn.Write(frame); err != nil {
		return nil, fmt.Errorf("attach proxy send: %w", err)
	}

	// Read response: [1B status][4B payload_len][payload]
	var resp [5]byte
	if _, err := io.ReadFull(s.conn, resp[:]); err != nil {
		return nil, fmt.Errorf("attach proxy recv: %w", err)
	}
	status := resp[0]
	payloadLen := binary.BigEndian.Uint32(resp[1:])

	var payload []byte
	if payloadLen > 0 {
		if payloadLen > 65536 {
			return nil, fmt.Errorf("attach proxy: payload too large (%d)", payloadLen)
		}
		payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(s.conn, payload); err != nil {
			return nil, fmt.Errorf("attach proxy recv: %w", err)
		}
	}

	switch status {
	case 0:
		return payload, nil
	case 1:
		return nil, &proxyTimeoutError{}
	default:
		return nil, fmt.Errorf("attach proxy: %s", string(payload))
	}
}

// proxyTimeoutError satisfies net.Error so trace timeout checks work.
type proxyTimeoutError struct{}

func (e *proxyTimeoutError) Error() string   { return "attach proxy: recv timeout" }
func (e *proxyTimeoutError) Timeout() bool   { return true }
func (e *proxyTimeoutError) Temporary() bool { return true }
