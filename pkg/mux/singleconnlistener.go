package mux

import (
	"io"
	"net"
	"sync"
)

type singleConnListener struct {
	conn net.Conn
	done bool
	l    sync.Mutex
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	l.l.Lock()
	if l.done {
		l.l.Unlock()
		return nil, io.ErrClosedPipe
	}
	defer l.l.Unlock()

	l.done = true

	return l.conn, nil
}

func (l *singleConnListener) Addr() net.Addr {
	return l.conn.RemoteAddr()
}

func (l *singleConnListener) Close() error {
	return nil
}
