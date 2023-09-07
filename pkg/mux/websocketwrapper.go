package mux

import (
	"net"
	"time"

	"golang.org/x/net/websocket"
)

type websocketWrapper struct {
	wsConn  *websocket.Conn
	tcpConn net.Conn
}

func (ww *websocketWrapper) Read(b []byte) (n int, err error) {
	return ww.wsConn.Read(b)
}

func (ww *websocketWrapper) Write(b []byte) (n int, err error) {
	return ww.wsConn.Write(b)
}

func (ww *websocketWrapper) Close() error {
	return ww.wsConn.Close()
}

func (ww *websocketWrapper) LocalAddr() net.Addr {
	return ww.tcpConn.LocalAddr()
}

func (ww *websocketWrapper) RemoteAddr() net.Addr {
	return ww.tcpConn.RemoteAddr()
}

func (ww *websocketWrapper) SetDeadline(t time.Time) error {
	return ww.wsConn.SetDeadline(t)
}

func (ww *websocketWrapper) SetReadDeadline(t time.Time) error {
	return ww.wsConn.SetReadDeadline(t)
}

func (ww *websocketWrapper) SetWriteDeadline(t time.Time) error {
	return ww.wsConn.SetWriteDeadline(t)
}
