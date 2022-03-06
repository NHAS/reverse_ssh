package mux

import (
	"bytes"
	"errors"
	"net"
	"time"
)

type MultiplexerConfig struct {
	SSH  bool
	HTTP bool
}

type Multiplexer struct {
	protocols map[string]*multiplexerListener
	done      bool
	listener  net.Listener
}

func ListenWithConfig(network, address string, c MultiplexerConfig) (*Multiplexer, error) {

	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}

	var m Multiplexer

	m.listener = listener

	m.protocols = map[string]*multiplexerListener{}

	if c.SSH {
		m.protocols["ssh"] = newMultiplexerListener(listener.Addr())
	}

	if c.HTTP {
		m.protocols["http"] = newMultiplexerListener(listener.Addr())
	}

	go func() {
		for !m.done {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}

			conn.SetDeadline(time.Now().Add(30 * time.Second))
			l, prefix, err := m.determineProtocol(conn)
			if err != nil {
				conn.Close()
				continue
			}

			conn.SetDeadline(time.Time{})

			go func() { l.connections <- &bufferedConn{conn: conn, prefix: prefix} }()
		}
	}()

	return &m, nil
}

func Listen(network, address string) (*Multiplexer, error) {
	c := MultiplexerConfig{
		SSH:  true,
		HTTP: true,
	}

	return ListenWithConfig(network, address, c)
}

func (m *Multiplexer) Close() {
	m.done = true
	m.listener.Close()
	for _, v := range m.protocols {
		v.Close()
	}

}

func (m *Multiplexer) determineProtocol(c net.Conn) (*multiplexerListener, []byte, error) {
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
