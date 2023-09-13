package mux

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"log"
	"math/big"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"
)

type MultiplexerConfig struct {
	SSH  bool
	HTTP bool

	TLS               bool
	AutoTLSCommonName string

	TLSCertPath string
	TLSKeyPath  string

	TcpKeepAlive int

	tlsConfig *tls.Config
}

// https://gist.github.com/shivakar/cd52b5594d4912fbeb46
func genX509KeyPair(AutoTLSCommonName string) (tls.Certificate, error) {
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.Unix()),
		Subject: pkix.Name{
			CommonName:   AutoTLSCommonName,
			Country:      []string{"US"},
			Organization: []string{"Cloudflare, Inc"},
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(0, 0, 30), // Valid for 30 days
		SubjectKeyId:          []byte{113, 117, 105, 99, 107, 115, 101, 114, 118, 101},
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	cert, err := x509.CreateCertificate(rand.Reader, template, template,
		priv.Public(), priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	var outCert tls.Certificate
	outCert.Certificate = append(outCert.Certificate, cert)
	outCert.PrivateKey = priv

	return outCert, nil
}

type Multiplexer struct {
	sync.RWMutex
	protocols      map[string]*multiplexerListener
	done           bool
	listeners      map[string]net.Listener
	newConnections chan net.Conn

	config MultiplexerConfig
}

func (m *Multiplexer) StartListener(network, address string) error {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.listeners[address]; ok {
		return errors.New("Address " + address + " already listening")
	}

	d := time.Duration(time.Duration(m.config.TcpKeepAlive) * time.Second)
	if m.config.TcpKeepAlive == 0 {
		d = time.Duration(-1)
	}

	lc := net.ListenConfig{
		KeepAlive: d,
	}

	listener, err := lc.Listen(context.Background(), network, address)
	if err != nil {
		return err
	}

	m.listeners[address] = listener

	go func(listen net.Listener) {
		for {
			// Raw TCP connection
			conn, err := listen.Accept()
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					m.Lock()

					delete(m.listeners, address)

					m.Unlock()
					return
				}
				continue

			}
			go func() {
				select {
				case m.newConnections <- conn:
				case <-time.After(2 * time.Second):
					log.Println("Accepting new connection timed out")
					conn.Close()
				}
			}()
		}

	}(listener)

	return nil
}

func (m *Multiplexer) StopListener(address string) error {
	m.Lock()
	defer m.Unlock()

	listener, ok := m.listeners[address]
	if !ok {
		return errors.New("Address " + address + " not listening")
	}

	return listener.Close()
}

func (m *Multiplexer) GetListeners() []string {
	m.RLock()
	defer m.RUnlock()

	listeners := []string{}
	for l := range m.listeners {
		listeners = append(listeners, l)
	}

	sort.Strings(listeners)

	return listeners
}

func (m *Multiplexer) QueueConn(c net.Conn) error {
	select {
	case m.newConnections <- c:
		return nil
	case <-time.After(250 * time.Millisecond):
		return errors.New("too busy to queue connection")
	}
}

func ListenWithConfig(network, address string, _c MultiplexerConfig) (*Multiplexer, error) {

	var m Multiplexer

	m.newConnections = make(chan net.Conn)
	m.listeners = make(map[string]net.Listener)
	m.protocols = map[string]*multiplexerListener{}
	m.config = _c

	err := m.StartListener(network, address)
	if err != nil {
		return nil, err
	}

	if m.config.SSH {
		m.protocols["ssh"] = newMultiplexerListener(m.listeners[address].Addr(), "ssh")
	}

	if m.config.HTTP {
		m.protocols["http"] = newMultiplexerListener(m.listeners[address].Addr(), "http")
	}

	var waitingConnections int32
	go func() {
		for conn := range m.newConnections {

			if atomic.LoadInt32(&waitingConnections) > 1000 {
				conn.Close()
				continue
			}

			//Atomic as other threads may be writing and reading while we do this
			atomic.AddInt32(&waitingConnections, 1)
			go func(conn net.Conn) {

				defer atomic.AddInt32(&waitingConnections, -1)

				conn.SetDeadline(time.Now().Add(2 * time.Second))

				var proto string
				conn, proto, err = m.determineProtocol(conn)
				if err != nil {
					log.Println("Multiplexing failed: ", err)
					return
				}

				if m.config.TLS && proto == "tls" {

					if m.config.tlsConfig == nil {

						tlsConfig := &tls.Config{
							PreferServerCipherSuites: true,
							CurvePreferences: []tls.CurveID{
								tls.CurveP256,
								tls.X25519, // Go 1.8 only
							},
							MinVersion: tls.VersionTLS12,
						}

						if m.config.TLSCertPath != "" {
							cert, err := tls.LoadX509KeyPair(m.config.TLSCertPath, m.config.TLSKeyPath)
							if err != nil {

								log.Println("TLS is enabled but loading certs/key failed: ", m.config.TLSCertPath, " err: ", err)
								return
							}

							tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
						} else {
							cert, err := genX509KeyPair(m.config.AutoTLSCommonName)
							if err != nil {
								log.Println("TLS is enabled but generating certs/key failed: ", err)
								return
							}
							tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
						}

						m.config.tlsConfig = tlsConfig
					}

					// this is TLS so replace the connection
					c := tls.Server(conn, m.config.tlsConfig)
					err := c.Handshake()
					if err != nil {
						conn.Close()

						if !strings.Contains(err.Error(), "remote error: tls: bad certificate") {
							log.Println("Multiplexing failed (tls handshake): ", err)
						}
						return
					}

					conn = c

				}

				conn.SetDeadline(time.Time{})

				functionalConn, proto, err := m.determineProtocol(conn)
				if err != nil {
					conn.Close()
					log.Println("Error determining functional protocol: ", err)
					return
				}

				if proto == "ws" {
					wsHttp := http.NewServeMux()
					wsConnChan := make(chan net.Conn, 1)

					wsServer := websocket.Server{
						Config: websocket.Config{},

						// Disable origin validation because.... its ssh we dont need it
						Handshake: nil,
						Handler: func(c *websocket.Conn) {
							// Pain and suffering https://github.com/golang/go/issues/7350
							c.PayloadType = websocket.BinaryFrame

							wsW := websocketWrapper{
								wsConn:  c,
								tcpConn: conn,
								done:    make(chan interface{}),
							}

							wsConnChan <- &wsW

							<-wsW.done
						},
					}

					wsHttp.Handle("/ws", wsServer)

					go http.Serve(&singleConnListener{conn: functionalConn}, wsHttp)

					select {
					case wsConn := <-wsConnChan:
						functionalConn, proto, err = m.determineProtocol(wsConn)
						if err != nil {
							wsConn.Close()
							log.Println("failed to determine protocol via ws: ", err)
							return
						}

					case <-time.After(2 * time.Second):
						conn.Close()
						log.Println("Multiplexing failed: websockets took too long to negotiate")
						return
					}
				}

				l, ok := m.protocols[proto]
				if !ok {
					functionalConn.Close()
					log.Println("Multiplexing failed: ", proto)
					return
				}

				select {
				//Allow whatever we're multiplexing to apply backpressure if it cant accept things
				case l.connections <- functionalConn:
				case <-time.After(2 * time.Second):

					log.Println(l.protocol, "Failed to accept new connection within 2 seconds, closing connection (may indicate high resource usage)")
					functionalConn.Close()
				}

			}(conn)

		}
	}()

	return &m, nil
}

func Listen(network, address string) (*Multiplexer, error) {
	c := MultiplexerConfig{
		SSH:          true,
		HTTP:         true,
		TcpKeepAlive: 7200, // Linux default timeout is 2 hours
	}

	return ListenWithConfig(network, address, c)
}

func (m *Multiplexer) Close() {
	m.done = true

	for address := range m.listeners {
		m.StopListener(address)
	}

	for _, v := range m.protocols {
		v.Close()
	}

	close(m.newConnections)

}

func isHttp(b []byte) bool {

	validMethods := [][]byte{
		[]byte("GET"), []byte("HEA"), []byte("POS"),
		[]byte("PUT"), []byte("DEL"), []byte("CON"),
		[]byte("OPT"), []byte("TRA"), []byte("PAT"),
	}

	for _, vm := range validMethods {
		if bytes.HasPrefix(b, vm) {
			return true
		}
	}

	return false
}

func (m *Multiplexer) determineProtocol(conn net.Conn) (net.Conn, string, error) {

	header := make([]byte, 7)
	n, err := conn.Read(header)
	if err != nil {
		return nil, "", err
	}

	c := &bufferedConn{prefix: header[:n], conn: conn}

	if bytes.HasPrefix(header, []byte{0x16}) {
		return c, "tls", nil
	}

	if bytes.HasPrefix(header, []byte{'S', 'S', 'H'}) {
		return c, "ssh", nil
	}

	if isHttp(header) {
		if bytes.HasPrefix(header, []byte("GET /ws")) {
			return c, "ws", nil
		}

		return c, "http", nil
	}

	return nil, "", errors.New("unknown protocol")
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
