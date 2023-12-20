package client

import (
	"net"
	"os"
	"time"
)

type InetdConn struct {
}

func (c *InetdConn) Read(b []byte) (n int, err error) {
	return os.Stdin.Read(b)
}

func (c *InetdConn) Write(b []byte) (n int, err error) {
	return os.Stdout.Write(b)
}
func (c *InetdConn) Close() error {
	os.Stdout.Close()
	return os.Stdin.Close()
}

func (c *InetdConn) LocalAddr() net.Addr {
	return &net.IPAddr{IP: net.IPv4(127, 0, 0, 1), Zone: ""}
}

func (c *InetdConn) RemoteAddr() net.Addr {
	return c.LocalAddr()
}

func (c *InetdConn) SetDeadline(t time.Time) error {
	os.Stdin.SetDeadline(t)
	return os.Stdout.SetDeadline(t)
}

func (c *InetdConn) SetReadDeadline(t time.Time) error {
	return os.Stdin.SetWriteDeadline(t)
}

func (c *InetdConn) SetWriteDeadline(t time.Time) error {
	return os.Stdout.SetWriteDeadline(t)
}
