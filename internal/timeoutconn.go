package internal

import (
	"net"
	"time"
)

type TimeoutConn struct {
	net.Conn
	Timeout time.Duration
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
