package mux

import (
	"net"
	"net/http"
	"strings"
)

type ConnectionMetadata struct {
	Transport     string
	RealClientIP  string
	ProxySourceIP string
}

type metadataConn struct {
	net.Conn
	metadata   ConnectionMetadata
	remoteAddr net.Addr
}

func (c *metadataConn) RemoteAddr() net.Addr {
	if c.remoteAddr != nil {
		return c.remoteAddr
	}
	return c.Conn.RemoteAddr()
}

func (c *metadataConn) Metadata() ConnectionMetadata {
	return c.metadata
}

func Metadata(conn net.Conn) ConnectionMetadata {
	if conn == nil {
		return ConnectionMetadata{}
	}
	if carrier, ok := conn.(interface{ Metadata() ConnectionMetadata }); ok {
		return carrier.Metadata()
	}
	return ConnectionMetadata{}
}

func withMetadata(conn net.Conn, metadata ConnectionMetadata) net.Conn {
	if conn == nil {
		return nil
	}
	remoteAddr := conn.RemoteAddr()
	if metadata.RealClientIP != "" {
		if realAddr := tcpAddrForIP(metadata.RealClientIP); realAddr != nil {
			remoteAddr = realAddr
		}
	}
	return &metadataConn{Conn: conn, metadata: metadata, remoteAddr: remoteAddr}
}

func (m *Multiplexer) metadataFromRequest(req *http.Request, source net.Addr, transportName string) ConnectionMetadata {
	metadata := ConnectionMetadata{Transport: transportName}
	sourceIP := addrIP(source)
	if sourceIP == nil || len(m.config.TrustedProxyCIDRs) == 0 || !ipInCIDRs(sourceIP, m.config.TrustedProxyCIDRs) {
		return metadata
	}

	realIP := net.ParseIP(strings.TrimSpace(req.Header.Get("X-Real-IP")))
	if realIP == nil {
		realIP = firstValidForwardedIP(req.Header.Get("X-Forwarded-For"))
	}
	if realIP == nil {
		return metadata
	}

	metadata.ProxySourceIP = sourceIP.String()
	metadata.RealClientIP = realIP.String()
	return metadata
}

func addrIP(addr net.Addr) net.IP {
	if addr == nil {
		return nil
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}
	return net.ParseIP(strings.Trim(host, "[]"))
}

func firstValidForwardedIP(header string) net.IP {
	for _, part := range strings.Split(header, ",") {
		if ip := net.ParseIP(strings.TrimSpace(part)); ip != nil {
			return ip
		}
	}
	return nil
}

func ipInCIDRs(ip net.IP, cidrs []*net.IPNet) bool {
	for _, cidr := range cidrs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func tcpAddrForIP(ip string) net.Addr {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil
	}
	return &net.TCPAddr{IP: parsed, Port: 0}
}
