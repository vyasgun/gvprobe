package attach

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"

	"github.com/vyasgun/gvprobe/pkg/gvproxy/datapath"
)

// serveProxy accepts trace clients on ln and forwards frames through s.
func serveProxy(ctx context.Context, ln net.Listener, mu *sync.Mutex, s datapath.Session) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		go handleProxyConn(ctx, conn, mu, s)
	}
}

// handleProxyConn reads frame requests from a trace client, forwards them
// through the held datapath session, and writes back the reply.
//
// Wire format per round-trip:
//
//	Request:  [4B BE timeout_ms][4B BE frame_len][frame]
//	Response: [1B status][4B BE payload_len][payload]
//	  status 0 = reply frame
//	  status 1 = recv timeout (empty payload)
//	  status 2 = error (payload = message)
func handleProxyConn(ctx context.Context, conn net.Conn, mu *sync.Mutex, s datapath.Session) {
	defer func() { _ = conn.Close() }()
	for {
		if ctx.Err() != nil {
			return
		}

		// Read request header: timeout + frame length.
		var hdr [8]byte
		if _, err := io.ReadFull(conn, hdr[:]); err != nil {
			return
		}
		timeoutMs := binary.BigEndian.Uint32(hdr[:4])
		frameLen := binary.BigEndian.Uint32(hdr[4:])
		if frameLen == 0 || frameLen > 65536 {
			return
		}

		frame := make([]byte, frameLen)
		if _, err := io.ReadFull(conn, frame); err != nil {
			return
		}

		timeout := time.Duration(timeoutMs) * time.Millisecond

		mu.Lock()
		sendErr := s.SendFrame(frame)
		var reply []byte
		var recvErr error
		if sendErr == nil {
			reply, recvErr = s.RecvFrame(timeout)
		}
		mu.Unlock()

		switch {
		case sendErr != nil:
			_ = writeProxyResp(conn, 2, []byte(sendErr.Error()))
		case recvErr != nil:
			if ne, ok := recvErr.(net.Error); ok && ne.Timeout() {
				_ = writeProxyResp(conn, 1, nil)
			} else {
				_ = writeProxyResp(conn, 2, []byte(recvErr.Error()))
			}
		default:
			_ = writeProxyResp(conn, 0, reply)
		}
	}
}

func writeProxyResp(conn net.Conn, status byte, payload []byte) error {
	var hdr [5]byte
	hdr[0] = status
	binary.BigEndian.PutUint32(hdr[1:], uint32(len(payload)))
	if _, err := conn.Write(hdr[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err := conn.Write(payload)
		return err
	}
	return nil
}
