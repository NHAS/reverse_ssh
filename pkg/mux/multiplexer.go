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
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NHAS/reverse_ssh/pkg/mux/protocols"
	"golang.org/x/net/websocket"
)

type MultiplexerConfig struct {
	Control   bool
	Downloads bool

	TLS               bool
	AutoTLSCommonName string

	TLSCertPath string
	TLSKeyPath  string

	TcpKeepAlive int

	PollingAuthChecker func(key string, addr net.Addr) bool

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
	result         map[protocols.Type]*multiplexerListener
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

type ConnContextKey string

var contextKey ConnContextKey = "conn"

func (m *Multiplexer) startHttpServer() {
	listener := m.getProtoListener(protocols.HTTP)

	go func(l net.Listener) {

		srv := &http.Server{
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 60 * time.Second,
			Handler:      m.collector(listener.Addr()),
			ConnContext: func(ctx context.Context, c net.Conn) context.Context {
				return context.WithValue(ctx, contextKey, c)
			},
		}

		log.Println(srv.Serve(l))
	}(listener)
}

func (m *Multiplexer) collector(localAddr net.Addr) http.HandlerFunc {

	var (
		// key to number of devices using that key
		connections = map[string]*fragmentedConnection{}
		lck         sync.Mutex
	)

	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodHead && req.Method != http.MethodGet && req.Method != http.MethodPost {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		lck.Lock()

		defer req.Body.Close()

		id := req.URL.Query().Get("id")
		c, ok := connections[id]
		if !ok {
			defer lck.Unlock()

			if req.Method == http.MethodHead {

				if len(connections) > 2000 {
					log.Println("server has too many polling connections (", len(connections), " limit is 2k")
					http.Error(w, "Server Error", http.StatusInternalServerError)
					return
				}

				key := req.URL.Query().Get("key")

				var err error

				realConn, ok := req.Context().Value(contextKey).(net.Conn)
				if !ok {
					log.Println("couldnt get real connection address")
					http.Error(w, "Server Error", http.StatusInternalServerError)
					return
				}

				if !m.config.PollingAuthChecker(key, realConn.RemoteAddr()) {
					log.Println("client connected but the key for starting a new polling session was wrong")
					http.Error(w, "Bad Request", http.StatusBadRequest)
					return
				}

				c, id, err = NewFragmentCollector(localAddr, realConn.RemoteAddr(), func() {
					delete(connections, id)
				})
				if err != nil {
					log.Println("error generating new fragment collector: ", err)
					http.Error(w, "Server Error", http.StatusInternalServerError)

					return
				}

				connections[id] = c
				http.SetCookie(w, &http.Cookie{
					Name:  "NID",
					Value: id,
				})

				l := m.result[protocols.C2]
				select {
				//Allow whatever we're multiplexing to apply backpressure if it cant accept things
				case l.connections <- c:
				case <-time.After(2 * time.Second):

					log.Println(l.protocol, "Failed to accept new http connection within 2 seconds, closing connection (may indicate high resource usage)")
					c.Close()
					delete(connections, id)
					http.Error(w, "Server Error", http.StatusInternalServerError)
					return
				}

				http.Redirect(w, req, "/notification", http.StatusTemporaryRedirect)
				return

			}

			log.Println("client connected but did not have a valid session id")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		lck.Unlock()

		// Reset last seen time.
		c.IsAlive()

		switch req.Method {

		// Get any buffered/queued data
		case http.MethodGet:

			_, err := io.Copy(w, c.writeBuffer)
			if err != nil {
				if err == io.EOF {
					return
				}
				c.Close()
			}

		// Add data
		case http.MethodPost:
			_, err := io.Copy(c.readBuffer, req.Body)
			if err != nil {
				if err == io.EOF {
					return
				}
				c.Close()
			}
		}

	}
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
	m.result = map[protocols.Type]*multiplexerListener{}
	m.config = _c

	if _c.PollingAuthChecker == nil {
		return nil, errors.New("no authentication method supplied for polling muxing, this may lead to extreme dos if not set. Must set it")
	}

	err := m.StartListener(network, address)
	if err != nil {
		return nil, err
	}

	if m.config.Control {
		m.result[protocols.C2] = newMultiplexerListener(m.listeners[address].Addr(), protocols.C2)
	}

	if m.config.Downloads {
		m.result[protocols.HTTPDownload] = newMultiplexerListener(m.listeners[address].Addr(), protocols.HTTPDownload)
		m.result[protocols.TCPDownload] = newMultiplexerListener(m.listeners[address].Addr(), protocols.TCPDownload)
	}

	m.result[protocols.HTTP] = newMultiplexerListener(m.listeners[address].Addr(), protocols.HTTP)

	// Starts the composer http server turns a bunch of posts/gets into a coherent connection
	m.startHttpServer()

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

				newConnection, proto, err := m.unwrapTransports(conn)
				if err != nil {
					log.Println("Multiplexing failed (unwrapping): ", err)
					return
				}

				l, ok := m.result[proto]
				if !ok {
					newConnection.Close()
					log.Println("Multiplexing failed (final determination): ", proto)
					return
				}

				select {
				//Allow whatever we're multiplexing to apply backpressure if it cant accept things
				case l.connections <- newConnection:
				case <-time.After(2 * time.Second):

					log.Println(l.protocol, "Failed to accept new connection within 2 seconds, closing connection (may indicate high resource usage)")
					newConnection.Close()
				}

			}(conn)

		}
	}()

	return &m, nil
}

func Listen(network, address string) (*Multiplexer, error) {
	c := MultiplexerConfig{
		Control:      true,
		Downloads:    true,
		TcpKeepAlive: 7200, // Linux default timeout is 2 hours
	}

	return ListenWithConfig(network, address, c)
}

func (m *Multiplexer) Close() {
	m.done = true

	for address := range m.listeners {
		m.StopListener(address)
	}

	for _, v := range m.result {
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

func (m *Multiplexer) determineProtocol(conn net.Conn) (net.Conn, protocols.Type, error) {

	header := make([]byte, 14)
	n, err := conn.Read(header)
	if err != nil {
		conn.Close()
		return nil, "", fmt.Errorf("failed to read header: %s", err)
	}

	c := &bufferedConn{prefix: header[:n], conn: conn}

	if bytes.HasPrefix(header, []byte{'R', 'A', 'W'}) {
		return c, protocols.TCPDownload, nil
	}

	if bytes.HasPrefix(header, []byte{0x16}) {
		return c, protocols.TLS, nil
	}

	if bytes.HasPrefix(header, []byte{'S', 'S', 'H'}) {
		return c, protocols.C2, nil
	}

	if isHttp(header) {

		if bytes.HasPrefix(header, []byte("GET /ws")) {
			return c, protocols.Websockets, nil
		}

		if bytes.HasPrefix(header, []byte("HEAD /push")) || bytes.HasPrefix(header, []byte("GET /push")) || bytes.HasPrefix(header, []byte("POST /push")) {
			return c, protocols.HTTP, nil
		}

		return c, protocols.HTTPDownload, nil
	}

	conn.Close()
	return nil, "", errors.New("unknown protocol: " + string(header[:n]))
}

func (m *Multiplexer) getProtoListener(proto protocols.Type) net.Listener {
	ml, ok := m.result[proto]
	if !ok {
		panic("Unknown protocol passed: " + proto)
	}

	return ml
}

func (m *Multiplexer) unwrapTransports(conn net.Conn) (net.Conn, protocols.Type, error) {
	conn.SetDeadline(time.Now().Add(2 * time.Second))

	var proto protocols.Type
	conn, proto, err := m.determineProtocol(conn)
	if err != nil {
		return nil, protocols.Invalid, fmt.Errorf("initial determination: %s", err)
	}

	conn.SetDeadline(time.Time{})

	// Unwrap any outer tls if required
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
					return nil, protocols.Invalid, fmt.Errorf("TLS is enabled but loading certs/key failed: %s, err: %s", m.config.TLSCertPath, err)
				}

				tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
			} else {
				cert, err := genX509KeyPair(m.config.AutoTLSCommonName)
				if err != nil {
					return nil, protocols.Invalid, fmt.Errorf("TLS is enabled but generating certs/key failed: %s", err)
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
			return nil, protocols.Invalid, fmt.Errorf("multiplexing failed (tls handshake): err: %s", err)
		}

		// If we did unwrap tls, we now peek into the inner protocol to see whats there
		conn, proto, err = m.determineProtocol(c)
		if err != nil {
			return nil, protocols.Invalid, fmt.Errorf("error determining functional protocol: %s", err)
		}

	}

	switch proto {
	case protocols.Websockets:
		return m.unwrapWebsockets(conn)
	case protocols.HTTP:
		// This will get passed off to a golang stdlib http server to do further unwrapping/feeding to the ssh component.
		// Unlike the other connections this isnt a single stream, its multiple connections composed into one blob, so it has to be a lil non-standard
		return conn, protocols.HTTP, nil
	default:
		// If the initial unwrapping was enough and left us with download or ssh, we can just quit
		if protocols.FullyUnwrapped(proto) {
			return conn, proto, nil
		}
	}

	return nil, protocols.Invalid, fmt.Errorf("after unwrapping transports, nothing useable was found: %s", proto)
}

func (m *Multiplexer) unwrapWebsockets(conn net.Conn) (net.Conn, protocols.Type, error) {
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

	go http.Serve(&singleConnListener{conn: conn}, wsHttp)

	select {
	case wsConn := <-wsConnChan:
		// Determine if we're downloading a file over this ws connection, or connecting to ssh
		result, proto, err := m.determineProtocol(wsConn)
		if err != nil {
			conn.Close()
			return nil, protocols.Invalid, fmt.Errorf("failed to determine protocol being carried by ws: %s", err)
		}

		if !protocols.FullyUnwrapped(proto) {
			conn.Close()
			return nil, protocols.Invalid, errors.New("after unwrapping websockets found another protocol to unwrap (not control channel or download), does not support infinite protocol nesting")
		}

		return result, proto, nil

	case <-time.After(2 * time.Second):
		conn.Close()
		return nil, protocols.Invalid, errors.New("multiplexing failed: websockets took too long to negotiate")
	}
}

func (m *Multiplexer) ControlRequests() net.Listener {
	return m.getProtoListener(protocols.C2)
}

func (m *Multiplexer) HTTPDownloadRequests() net.Listener {
	return m.getProtoListener(protocols.HTTPDownload)
}

func (m *Multiplexer) TCPDownloadRequests() net.Listener {
	return m.getProtoListener(protocols.TCPDownload)
}
