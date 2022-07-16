package handlers

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

func Tun(_ *internal.User, newChannel ssh.NewChannel, l logger.Logger) {

	var tunInfo struct {
		Mode uint32
		No   uint32
	}

	err := ssh.Unmarshal(newChannel.ExtraData(), &tunInfo)
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, "connection closed")
		l.Warning("Unable to accept new channel %s", err)
		return
	}

	if tunInfo.Mode != 1 {
		newChannel.Reject(ssh.ConnectionFailed, "connection closed")
		return
	}

	tunnel, req, err := newChannel.Accept()
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, "connection closed")
		l.Warning("Unable to accept new channel %s", err)
		return
	}
	defer tunnel.Close()

	// Create a new gvisor userland network stack.
	ns := stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
			ipv6.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol,
			udp.NewProtocol,
			icmp.NewProtocol4,
			icmp.NewProtocol6,
		},
		HandleLocal: false,
	})

	linkEP := NewSSHEndpoint(tunnel)

	// Create a new NIC
	if err := ns.CreateNIC(1, linkEP); err != nil {
		log.Printf("CreateNIC: %v", err)
		return
	}

	// Gvisor Hack: Disable ICMP handling.
	ns.SetICMPLimit(0)
	ns.SetICMPBurst(0)

	// Forward TCP connections
	tcpHandler := tcp.NewForwarder(ns, 30000, 4000, forwardTCP)

	// Forward UDP connections
	udpHandler := udp.NewForwarder(ns, forwardUDP(ns))

	// Register forwarders
	ns.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpHandler.HandlePacket)
	ns.SetTransportProtocolHandler(udp.ProtocolNumber, udpHandler.HandlePacket)

	// Allow all routes by default
	ns.SetRouteTable([]tcpip.Route{
		{
			Destination: header.IPv4EmptySubnet,
			NIC:         1,
		},
		{
			Destination: header.IPv6EmptySubnet,
			NIC:         1,
		},
	})

	// Enable forwarding
	ns.SetForwardingDefaultAndAllNICs(ipv4.ProtocolNumber, true)
	ns.SetForwardingDefaultAndAllNICs(ipv6.ProtocolNumber, true)

	// Enable TCP SACK
	nsacks := tcpip.TCPSACKEnabled(true)
	ns.SetTransportProtocolOption(tcp.ProtocolNumber, &nsacks)

	// Disable SYN-Cookies, as this can mess with nmap scans
	synCookies := tcpip.TCPAlwaysUseSynCookies(false)
	ns.SetTransportProtocolOption(tcp.ProtocolNumber, &synCookies)

	// Allow packets from all sources/destinations
	ns.SetPromiscuousMode(1, true)
	ns.SetSpoofing(1, true)

	ssh.DiscardRequests(req)

	l.Info("Tunnel ended")

}

func forwardUDP(stack *stack.Stack) func(request *udp.ForwarderRequest) {
	return func(request *udp.ForwarderRequest) {

		id := request.ID()

		var wq waiter.Queue

		ep, iperr := request.CreateEndpoint(&wq)
		if iperr != nil {
			ep.Close()
			return
		}
		defer ep.Close()

		log.Printf("[+] %s -> %s:%d/udp\n", id.RemoteAddress, id.LocalAddress, id.LocalPort)

		fwdDst := net.UDPAddr{
			IP:   net.ParseIP(id.LocalAddress.String()),
			Port: int(id.LocalPort),
		}

		conn, err := net.Dial("udp", fwdDst.String())
		if err != nil {
			return
		}
		defer conn.Close()

		gonetConn := gonet.NewUDPConn(stack, &wq, ep)
		defer gonetConn.Close()

		go io.Copy(gonetConn, conn)
		_, err = io.Copy(conn, gonetConn)

	}
}

func forwardTCP(request *tcp.ForwarderRequest) {
	id := request.ID()

	var wq waiter.Queue
	ep, errTcp := request.CreateEndpoint(&wq)
	if errTcp != nil {
		fmt.Printf("r.CreateEndpoint() = %v\n", errTcp)
		request.Complete(true)
		return
	}

	request.Complete(false)

	c := gonet.NewTCPConn(&wq, ep)
	defer c.Close()

	fwdDst := net.TCPAddr{
		IP:   net.ParseIP(id.LocalAddress.String()),
		Port: int(id.LocalPort),
	}

	log.Printf("[+] %s -> %s:%d/tcp\n", id.RemoteAddress, id.LocalAddress, id.LocalPort)

	remote, err := net.Dial("tcp", fwdDst.String())
	if err != nil {
		c.Close()
		request.Complete(true)
		fmt.Println(err)
		return
	}
	defer remote.Close()
	defer c.Close()

	go io.Copy(remote, c)
	_, err = io.Copy(c, remote)
}

type SSHEndpoint struct {
	dispatcher stack.NetworkDispatcher
	tunnel     ssh.Channel
}

func NewSSHEndpoint(dev ssh.Channel) *SSHEndpoint {
	return &SSHEndpoint{
		tunnel: dev,
	}
}

// MTU implements stack.LinkEndpoint.
func (m *SSHEndpoint) MTU() uint32 {
	return 1500
}

// Capabilities implements stack.LinkEndpoint.
func (m *SSHEndpoint) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityNone
}

// MaxHeaderLength implements stack.LinkEndpoint.
func (m *SSHEndpoint) MaxHeaderLength() uint16 {
	return 0
}

// LinkAddress implements stack.LinkEndpoint.
func (m *SSHEndpoint) LinkAddress() tcpip.LinkAddress {
	return ""
}

// Attach implements stack.LinkEndpoint.
func (m *SSHEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	m.dispatcher = dispatcher
	go m.dispatchLoop()
}

func (m *SSHEndpoint) dispatchLoop() {
	for {
		packet := make([]byte, 1500)

		n, err := m.tunnel.Read(packet)
		if err != nil {
			break
		}

		//Remove the SSH added family address uint32 (for layer 3 tun)
		packet = packet[4:]

		if !m.IsAttached() {
			continue
		}

		pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.NewWithData(packet[:n-4]),
		})

		switch header.IPVersion(packet) {
		case header.IPv4Version:
			m.dispatcher.DeliverNetworkPacket(header.IPv4ProtocolNumber, pkb)
		case header.IPv6Version:
			m.dispatcher.DeliverNetworkPacket(header.IPv6ProtocolNumber, pkb)
		}
	}
}

// IsAttached implements stack.LinkEndpoint.
func (m *SSHEndpoint) IsAttached() bool {
	return m.dispatcher != nil
}

// WritePackets writes outbound packets
func (m *SSHEndpoint) WritePackets(pkts stack.PacketBufferList) (int, tcpip.Error) {
	n := 0
	for _, pkt := range pkts.AsSlice() {
		if err := m.WritePacket(pkt); err != nil {
			break
		}
		n++
	}
	return n, nil
}

// WritePacket writes outbound packets
func (m *SSHEndpoint) WritePacket(pkt *stack.PacketBuffer) tcpip.Error {
	var buf buffer.Buffer
	pktBuf := pkt.Buffer()
	buf.Merge(&pktBuf)

	// 3.2 Frame Format
	// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/Documentation/networking/tuntap.rst?id=HEAD
	packet := make([]byte, 4)
	binary.BigEndian.PutUint16(packet, 1)
	binary.BigEndian.PutUint16(packet[2:], uint16(pkt.NetworkProtocolNumber))

	packet = append(packet, buf.Flatten()...)

	if _, err := m.tunnel.Write(packet); err != nil {
		return &tcpip.ErrInvalidEndpointState{}
	}
	return nil
}

// Wait implements stack.LinkEndpoint.Wait.
func (m *SSHEndpoint) Wait() {}

// ARPHardwareType implements stack.LinkEndpoint.ARPHardwareType.
func (*SSHEndpoint) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

// AddHeader implements stack.LinkEndpoint.AddHeader.
func (*SSHEndpoint) AddHeader(pkt *stack.PacketBuffer) {
}

// WriteRawPacket implements stack.LinkEndpoint.
func (*SSHEndpoint) WriteRawPacket(*stack.PacketBuffer) tcpip.Error {
	return &tcpip.ErrNotSupported{}
}
