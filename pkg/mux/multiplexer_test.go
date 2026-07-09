package mux

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetadataFromRequestTrustsConfiguredProxy(t *testing.T) {
	_, trusted, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	m := &Multiplexer{config: MultiplexerConfig{TrustedProxyCIDRs: []*net.IPNet{trusted}}}
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.99, 198.51.100.10")
	req.Header.Set("X-Real-IP", "198.51.100.10")

	metadata := m.metadataFromRequest(req, &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 4444}, "wss")
	if metadata.Transport != "wss" {
		t.Fatalf("transport = %q", metadata.Transport)
	}
	if metadata.RealClientIP != "198.51.100.10" {
		t.Fatalf("real client ip = %q", metadata.RealClientIP)
	}
	if metadata.ProxySourceIP != "192.0.2.10" {
		t.Fatalf("proxy source ip = %q", metadata.ProxySourceIP)
	}
}

func TestMetadataFromRequestFallsBackToForwardedFor(t *testing.T) {
	_, trusted, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	m := &Multiplexer{config: MultiplexerConfig{TrustedProxyCIDRs: []*net.IPNet{trusted}}}
	req := httptest.NewRequest(http.MethodGet, "/push", nil)
	req.Header.Set("X-Forwarded-For", "bad-ip, 198.51.100.10")

	metadata := m.metadataFromRequest(req, &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 4444}, "http")
	if metadata.RealClientIP != "198.51.100.10" {
		t.Fatalf("real client ip = %q", metadata.RealClientIP)
	}
}

func TestMetadataFromRequestIgnoresUntrustedProxy(t *testing.T) {
	_, trusted, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	m := &Multiplexer{config: MultiplexerConfig{TrustedProxyCIDRs: []*net.IPNet{trusted}}}
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("X-Real-IP", "198.51.100.10")

	metadata := m.metadataFromRequest(req, &net.TCPAddr{IP: net.ParseIP("203.0.113.10"), Port: 4444}, "wss")
	if metadata.Transport != "wss" {
		t.Fatalf("transport = %q", metadata.Transport)
	}
	if metadata.RealClientIP != "" {
		t.Fatalf("untrusted real client ip = %q", metadata.RealClientIP)
	}
	if metadata.ProxySourceIP != "" {
		t.Fatalf("untrusted proxy source ip = %q", metadata.ProxySourceIP)
	}
}

func TestBufferedConnCarriesMetadata(t *testing.T) {
	conn := withMetadata(&testConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 4444}}, ConnectionMetadata{
		Transport:     "wss",
		RealClientIP:  "198.51.100.10",
		ProxySourceIP: "192.0.2.10",
	})
	buffered := &bufferedConn{conn: conn}

	metadata := Metadata(buffered)
	if metadata.Transport != "wss" || metadata.ProxySourceIP != "192.0.2.10" {
		t.Fatalf("metadata lost: %+v", metadata)
	}
	if got := buffered.RemoteAddr().String(); got != "198.51.100.10:0" {
		t.Fatalf("remote addr = %q", got)
	}
}

func TestFragmentedConnCarriesMetadata(t *testing.T) {
	metadata := ConnectionMetadata{
		Transport:     "http",
		RealClientIP:  "198.51.100.10",
		ProxySourceIP: "192.0.2.10",
	}
	conn, _, err := NewFragmentCollector(
		&net.TCPAddr{IP: net.ParseIP("192.0.2.20"), Port: 3232},
		&net.TCPAddr{IP: net.ParseIP("198.51.100.10"), Port: 0},
		metadata,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if got := Metadata(conn); got != metadata {
		t.Fatalf("metadata = %+v, want %+v", got, metadata)
	}
}

type testConn struct {
	net.Conn
	remoteAddr net.Addr
}

func (c *testConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}
