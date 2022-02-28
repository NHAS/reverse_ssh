package server

import (
	"bytes"
	"errors"
	"net"
	"time"

	"github.com/NHAS/reverse_ssh/pkg/logger"
)

type bufferedConn struct {
	prefix []byte
	conn   net.Conn
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

type MultiplexerConfig struct {
	SSH  bool
	HTTP bool
	Log  logger.Logger
}

type MultiplexerListener struct {
	addr        net.Addr
	connections chan net.Conn
	closed      bool
}

func NewMultiplexerListener(addr net.Addr) *MultiplexerListener {
	return &MultiplexerListener{addr: addr, connections: make(chan net.Conn)}
}

func (ml *MultiplexerListener) Accept() (net.Conn, error) {
	if ml.closed {
		return nil, errors.New("Accept on closed listener")
	}
	return <-ml.connections, nil
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (ml *MultiplexerListener) Close() error {
	if !ml.closed {
		ml.closed = true
		close(ml.connections)
	}

	return nil
}

// Addr returns the listener's network address.
func (ml *MultiplexerListener) Addr() net.Addr {
	if ml.closed {
		return nil
	}
	return ml.addr
}

type Multiplexer struct {
	protocols map[string]*MultiplexerListener
	done      bool
	listener  net.Listener
}

func (m *Multiplexer) Listen(network, address string, c MultiplexerConfig) error {
	listener, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	m.listener = listener

	m.protocols = map[string]*MultiplexerListener{}

	if c.SSH {
		m.protocols["ssh"] = NewMultiplexerListener(listener.Addr())
	}

	if c.HTTP {
		m.protocols["http"] = NewMultiplexerListener(listener.Addr())
	}

	go func() {
		for !m.done {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}
			defer conn.Close()

			l, prefix, err := m.determineProtocol(conn)
			if err != nil {
				continue
			}

			go func() { l.connections <- &bufferedConn{conn: conn, prefix: prefix} }()
		}
	}()

	return nil
}

func (m *Multiplexer) Close() {
	m.done = true
	m.listener.Close()
	for _, v := range m.protocols {
		v.Close()
	}

}

func (m *Multiplexer) determineProtocol(c net.Conn) (*MultiplexerListener, []byte, error) {
	b := make([]byte, 3)
	_, err := c.Read(b)
	if err != nil {
		return nil, nil, err
	}

	proto := "http"
	if bytes.HasPrefix(b, []byte{'S', 'S', 'H'}) {
		proto = "ssh"
	}

	l, ok := m.protocols[proto]
	if !ok {
		return nil, nil, errors.New("Unknown protocol")
	}

	return l, b, nil
}

func (m *Multiplexer) getProtoListener(proto string) net.Listener {
	ml, ok := m.protocols[proto]
	if !ok {
		panic("Unknown protocol passed: " + proto)
	}

	return ml
}

func (m *Multiplexer) SSH() net.Listener {
	return m.getProtoListener("ssh")
}

func (m *Multiplexer) HTTP() net.Listener {
	return m.getProtoListener("http")
}
