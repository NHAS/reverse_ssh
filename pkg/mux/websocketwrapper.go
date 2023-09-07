package mux

import (
	"net"
	"time"

	"golang.org/x/net/websocket"
)

type websocketWrapper struct {
	wsConn  *websocket.Conn
	tcpConn net.Conn
	done    chan interface{}
}

func (ww *websocketWrapper) Read(b []byte) (n int, err error) {
	n, err = ww.wsConn.Read(b)
	if err != nil {
		ww.done <- true
	}
	return n, err
}

func (ww *websocketWrapper) Write(b []byte) (n int, err error) {
	n, err = ww.wsConn.Write(b)
	if err != nil {
		ww.done <- true
	}
	return
}

func (ww *websocketWrapper) Close() error {
	err := ww.wsConn.Close()
	ww.done <- true
	return err
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
