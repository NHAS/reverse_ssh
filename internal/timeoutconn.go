package internal

import (
	"net"
	"time"
)

type TimeoutConn struct {
	net.Conn
	Timeout time.Duration
}

func NewTimeoutConn(conn net.Conn, timeout time.Duration) *TimeoutConn {
	if timeout != 0 {
		conn.SetDeadline(time.Now().Add(timeout))
	}

	return &TimeoutConn{
		Conn:    conn,
		Timeout: timeout,
	}
}

func (c *TimeoutConn) Read(b []byte) (int, error) {

	if c.Timeout != 0 {
		c.Conn.SetDeadline(time.Now().Add(c.Timeout))
	}
	return c.Conn.Read(b)
}

func (c *TimeoutConn) Write(b []byte) (int, error) {
	if c.Timeout != 0 {
		c.Conn.SetDeadline(time.Now().Add(c.Timeout))
	}
	return c.Conn.Write(b)
}
