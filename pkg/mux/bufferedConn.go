package mux

import (
	"net"
	"time"
)

// PivotParenter is implemented by conns that were tunnelled through another client.
type PivotParenter interface {
	PivotParent() string
}

// GetPivotParent walks wrapper chains to find a pivot parent ID, if any.
func GetPivotParent(c net.Conn) string {
	type unwrapper interface {
		Unwrap() net.Conn
	}
	for i := 0; i < 10 && c != nil; i++ {
		if p, ok := c.(PivotParenter); ok {
			return p.PivotParent()
		}
		u, ok := c.(unwrapper)
		if !ok {
			break
		}
		c = u.Unwrap()
	}
	return ""
}

// tlsConnWrapper wraps a *tls.Conn and exposes Unwrap so GetPivotParent
// can reach through TLS to find a pivot parent.
type tlsConnWrapper struct {
	net.Conn
	inner net.Conn
}

func (t *tlsConnWrapper) Unwrap() net.Conn {
	return t.inner
}

type bufferedConn struct {
	prefix []byte
	conn   net.Conn
}

func (bc *bufferedConn) Unwrap() net.Conn {
	return bc.conn
}

func (bc *bufferedConn) Read(b []byte) (n int, err error) {
	if len(bc.prefix) > 0 {
		n = copy(b, bc.prefix)
		bc.prefix = bc.prefix[n:]
		return n, nil
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
