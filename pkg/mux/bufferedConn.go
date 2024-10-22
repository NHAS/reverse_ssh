package mux

import (
	"net"
	"time"
)

type bufferedConn struct {
	prefix []byte
	conn   net.Conn
}

func (bc *bufferedConn) Read(b []byte) (n int, err error) {
	if len(bc.prefix) > 0 {
		n = copy(b, bc.prefix)

		bc.prefix = bc.prefix[n:]

		var err error
		if len(b)-n > 0 {
			// If we havent exhausted the size of b, read some more
			var actualRead int
			actualRead, err = bc.conn.Read(b[n:])
			n += actualRead
		}

		return n, err
	}

	return bc.conn.Read(b)
}

func (bc *bufferedConn) Write(b []byte) (n int, err error) {
	return bc.conn.Write(b)
}

func (bc *bufferedConn) Close() error {
	return bc.conn.Close()
}

func (bc *bufferedConn) LocalAddr() net.Addr {
	return bc.conn.LocalAddr()
}

func (bc *bufferedConn) RemoteAddr() net.Addr {
	return bc.conn.RemoteAddr()
}

func (bc *bufferedConn) SetDeadline(t time.Time) error {
	return bc.conn.SetDeadline(t)
}

func (bc *bufferedConn) SetReadDeadline(t time.Time) error {
	return bc.conn.SetReadDeadline(t)
}

func (bc *bufferedConn) SetWriteDeadline(t time.Time) error {
	return bc.conn.SetWriteDeadline(t)
}
