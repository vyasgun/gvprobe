package datapath

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// qemuSession sends length-prefixed Ethernet frames (BE uint32 + payload).
type qemuSession struct {
	conn net.Conn
}

func (s *qemuSession) SendFrame(frame []byte) error {
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(frame)))
	if _, err := s.conn.Write(hdr); err != nil {
		return err
	}
	_, err := s.conn.Write(frame)
	return err
}

func (s *qemuSession) RecvFrame(timeout time.Duration) ([]byte, error) {
	_ = s.conn.SetReadDeadline(time.Now().Add(timeout))
	var hdr [4]byte
	if _, err := io.ReadFull(s.conn, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 || n > 65536 {
		return nil, fmt.Errorf("invalid qemu frame length: %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(s.conn, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
