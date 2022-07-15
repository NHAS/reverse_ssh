package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

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
	"gvisor.dev/gvisor/pkg/tcpip/transport/raw"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const (
	ICMP = 1
	TCP  = 6
	UDP  = 17
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

	go ssh.DiscardRequests(req)

	NewStack(StackSettings{MaxInflight: 4096, tun: tunnel})

	for {
		<-time.After(1 * time.Second)
	}

	l.Info("Tunnel ended")

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
			Payload: buffer.NewWithData(packet[:n]),
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

	if _, err := m.tunnel.Write(buf.Flatten()); err != nil {
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

// NetStack is the structure used to store the connection pool and the gvisor network stack
type NetStack struct {
	stack *stack.Stack
	sync.Mutex
}

type StackSettings struct {
	MaxInflight int
	tun         ssh.Channel
}

// NewStack registers a new GVisor Network Stack
func NewStack(settings StackSettings) *NetStack {
	ns := NetStack{}
	ns.new(settings)
	return &ns
}

// GetStack returns the current Gvisor stack.Stack object
func (s *NetStack) GetStack() *stack.Stack {
	return s.stack
}

// New creates a new userland network stack (using Gvisor) that listen on a tun interface.
func (s *NetStack) new(stackSettings StackSettings) *stack.Stack {

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

	s.stack = ns

	// Gvisor Hack: Disable ICMP handling.
	ns.SetICMPLimit(0)
	ns.SetICMPBurst(0)

	// Forward TCP connections
	tcpHandler := tcp.NewForwarder(ns, 0, stackSettings.MaxInflight, func(request *tcp.ForwarderRequest) {

		log.Println(request)

		go func() {
			var wq waiter.Queue

			ep, iperr := request.CreateEndpoint(&wq)
			if iperr != nil {
				log.Println(iperr)
				return
			}

			gonetConn := gonet.NewTCPConn(&wq, ep)
			defer gonetConn.Close()
			defer ep.Close()

			log.Println(io.Copy(gonetConn, gonetConn))

		}()
	})

	// Forward UDP connections
	udpHandler := udp.NewForwarder(ns, func(request *udp.ForwarderRequest) {

		log.Println(request)

		go func() {
			var wq waiter.Queue

			ep, iperr := request.CreateEndpoint(&wq)
			if iperr != nil {
				log.Println(iperr)
				return
			}

			gonetConn := gonet.NewUDPConn(s.stack, &wq, ep)
			defer gonetConn.Close()
			defer ep.Close()

			b := make([]byte, 1000)
			n, err := gonetConn.Read(b)
			fmt.Println(string(b[:n]), err)

		}()

	})

	// Register forwarders
	ns.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpHandler.HandlePacket)
	ns.SetTransportProtocolHandler(udp.ProtocolNumber, udpHandler.HandlePacket)

	linkEP := NewSSHEndpoint(stackSettings.tun)

	// Create a new NIC
	if err := ns.CreateNIC(1, linkEP); err != nil {
		panic(fmt.Errorf("CreateNIC: %v", err))
	}

	// Start a endpoint that will reply to ICMP echo queries
	if err := icmpResponder(s); err != nil {
		log.Fatal(err)
	}

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
	ns.SetForwardingDefaultAndAllNICs(ipv4.ProtocolNumber, false)
	ns.SetForwardingDefaultAndAllNICs(ipv6.ProtocolNumber, false)

	// Enable TCP SACK
	nsacks := tcpip.TCPSACKEnabled(false)
	ns.SetTransportProtocolOption(tcp.ProtocolNumber, &nsacks)

	// Disable SYN-Cookies, as this can mess with nmap scans
	synCookies := tcpip.TCPAlwaysUseSynCookies(false)
	ns.SetTransportProtocolOption(tcp.ProtocolNumber, &synCookies)

	// Allow packets from all sources/destinations
	ns.SetPromiscuousMode(1, true)
	ns.SetSpoofing(1, true)

	return ns
}

// handleICMP process incoming ICMP packets and, depending on the target host status, respond a ICMP ECHO Reply
// Please note that other ICMP messages are not yet supported.
// func handleICMP(nstack *stack.Stack, localConn TunConn, yamuxConn *yamux.Session) {
// 	pkt := localConn.GetICMP().Request
// 	v, ok := pkt.Data().PullUp(header.ICMPv4MinimumSize)
// 	if !ok {
// 		return
// 	}
// 	h := header.ICMPv4(v)
// 	if h.Type() == header.ICMPv4Echo {
// 		iph := header.IPv4(pkt.NetworkHeader().View())
// 		yamuxConnectionSession, err := yamuxConn.Open()
// 		if err != nil {
// 			log.Println(err)
// 			return
// 		}
// 		log.Println("Checking if %s is alive...\n", iph.DestinationAddress().String())
// 		icmpPacket := protocol.HostPingRequestPacket{Address: iph.DestinationAddress().String()}

// 		protocolEncoder := protocol.NewEncoder(yamuxConnectionSession)
// 		protocolDecoder := protocol.NewDecoder(yamuxConnectionSession)

// 		if err := protocolEncoder.Encode(protocol.Envelope{
// 			Type:    protocol.MessageHostPingRequest,
// 			Payload: icmpPacket,
// 		}); err != nil {
// 			log.Println(err)
// 			return
// 		}

// 		log.Println("Awaiting ping response...")
// 		if err := protocolDecoder.Decode(); err != nil {
// 			log.Println(err)
// 			return
// 		}

// 		response := protocolDecoder.Envelope.Payload
// 		reply := response.(protocol.HostPingResponsePacket)
// 		if reply.Alive {
// 			ProcessICMP(nstack, pkt)
// 		}

// 	}
// 	// Ignore other ICMPs
// 	return
// }

// func HandlePacket(nstack *stack.Stack, localConn TunConn, yamuxConn *yamux.Session) {

// 	var endpointID stack.TransportEndpointID
// 	var prototransport uint8
// 	var protonet uint8

// 	// Switching part
// 	switch localConn.Protocol {
// 	case tcp.ProtocolNumber:
// 		endpointID = localConn.GetTCP().EndpointID
// 		prototransport = protocol.TransportTCP
// 	case udp.ProtocolNumber:
// 		endpointID = localConn.GetUDP().EndpointID
// 		prototransport = protocol.TransportUDP
// 	case icmp.ProtocolNumber4:
// 		// ICMPs can't be relayed
// 		handleICMP(nstack, localConn, yamuxConn)
// 		return
// 	}

// 	if endpointID.LocalAddress.To4() != "" {
// 		protonet = protocol.Networkv4
// 	} else {
// 		protonet = protocol.Networkv6
// 	}

// 	log.Println("Got packet source : %s - endpointID : %s:%d", endpointID.RemoteAddress, endpointID.LocalAddress, endpointID.LocalPort)

// 	yamuxConnectionSession, err := yamuxConn.Open()
// 	if err != nil {
// 		log.Println(err)
// 		return
// 	}
// 	connectPacket := protocol.ConnectRequestPacket{
// 		Net:       protonet,
// 		Transport: prototransport,
// 		Address:   endpointID.LocalAddress.String(),
// 		Port:      endpointID.LocalPort,
// 	}

// 	protocolEncoder := protocol.NewEncoder(yamuxConnectionSession)
// 	protocolDecoder := protocol.NewDecoder(yamuxConnectionSession)

// 	if err := protocolEncoder.Encode(protocol.Envelope{
// 		Type:    protocol.MessageConnectRequest,
// 		Payload: connectPacket,
// 	}); err != nil {
// 		log.Println(err)
// 		return
// 	}

// 	log.Println("Awaiting response...")
// 	if err := protocolDecoder.Decode(); err != nil {
// 		if err != io.EOF {
// 			log.Println(err)
// 		}
// 		return
// 	}

// 	response := protocolDecoder.Envelope.Payload
// 	reply := response.(protocol.ConnectResponsePacket)
// 	if reply.Established {
// 		log.Println("Connection established on remote end!")
// 		go func() {
// 			var wq waiter.Queue
// 			if localConn.IsTCP() {
// 				ep, iperr := localConn.GetTCP().Request.CreateEndpoint(&wq)
// 				if iperr != nil {
// 					log.Println(iperr)
// 					localConn.Terminate(true)
// 					return
// 				}
// 				gonetConn := gonet.NewTCPConn(&wq, ep)
// 				go relay.StartRelay(yamuxConnectionSession, gonetConn)

// 			} else if localConn.IsUDP() {
// 				ep, iperr := localConn.GetUDP().Request.CreateEndpoint(&wq)
// 				if iperr != nil {
// 					log.Println(iperr)
// 					localConn.Terminate(false)
// 					return
// 				}

// 				gonetConn := gonet.NewUDPConn(nstack, &wq, ep)
// 				go relay.StartRelay(yamuxConnectionSession, gonetConn)
// 			}

// 		}()
// 	} else {
// 		localConn.Terminate(reply.Reset)

// 	}

// }

// icmpResponder handle ICMP packets coming to gvisor/netstack.
// Instead of responding to all ICMPs ECHO by default, we try to
// execute a ping on the Agent, and depending of the response, we
// send a ICMP reply back.
func icmpResponder(s *NetStack) error {

	var wq waiter.Queue
	rawProto, rawerr := raw.NewEndpoint(s.stack, ipv4.ProtocolNumber, icmp.ProtocolNumber4, &wq)
	if rawerr != nil {
		return errors.New("could not create raw endpoint")
	}
	if err := rawProto.Bind(tcpip.FullAddress{}); err != nil {
		return errors.New("could not bind raw endpoint")
	}
	go func() {
		we, ch := waiter.NewChannelEntry(waiter.ReadableEvents)
		wq.EventRegister(&we)
		for {
			var buff bytes.Buffer
			_, err := rawProto.Read(&buff, tcpip.ReadOptions{})

			if _, ok := err.(*tcpip.ErrWouldBlock); ok {
				// Wait for data to become available.
				select {
				case <-ch:
					_, err := rawProto.Read(&buff, tcpip.ReadOptions{})

					if err != nil {
						if _, ok := err.(*tcpip.ErrWouldBlock); ok {
							// Oh, a race condition?
							continue
						} else {
							// This is bad.
							panic(err)
						}
					}

					iph := header.IPv4(buff.Bytes())

					hlen := int(iph.HeaderLength())
					if buff.Len() < hlen {
						return
					}

					// Reconstruct a ICMP PacketBuffer from bytes.
					view := buffer.NewWithData(buff.Bytes())
					packetbuff := stack.NewPacketBuffer(stack.PacketBufferOptions{
						Payload:            view,
						ReserveHeaderBytes: hlen,
					})

					packetbuff.NetworkProtocolNumber = ipv4.ProtocolNumber
					packetbuff.TransportProtocolNumber = icmp.ProtocolNumber4
					packetbuff.NetworkHeader().Consume(hlen)
					// tunConn := TunConn{
					// 	Protocol: icmp.ProtocolNumber4,
					// 	Handler:  ICMPConn{Request: packetbuff},
					// }

					// if err := s.pool.Add(tunConn); err != nil {
					// 	s.Unlock()
					// 	log.Println(err)
					// 	continue // Unknown error, continue...
					// }
				}
			}

		}
	}()
	return nil
}

// ProcessICMP send back a ICMP echo reply from after receiving a echo request.
// This code come mostly from pkg/tcpip/network/ipv4/icmp.go
func ProcessICMP(nstack *stack.Stack, pkt *stack.PacketBuffer) {
	// (gvisor) pkg/tcpip/network/ipv4/icmp.go:174 - handleICMP

	// ICMP packets don't have their TransportHeader fields set. See
	// icmp/protocol.go:protocol.Parse for a full explanation.
	v, ok := pkt.Data().PullUp(header.ICMPv4MinimumSize)
	if !ok {
		return
	}
	h := header.ICMPv4(v)

	// Only do in-stack processing if the checksum is correct.
	if pkt.Data().AsRange().Checksum() != 0xffff {
		return
	}

	iph := header.IPv4(pkt.NetworkHeader().View())
	var newOptions header.IPv4Options

	// TODO(b/112892170): Meaningfully handle all ICMP types.
	switch h.Type() {
	case header.ICMPv4Echo:

		replyData := pkt.Data().AsRange().ToOwnedView()
		ipHdr := header.IPv4(pkt.NetworkHeader().View())

		localAddressBroadcast := pkt.NetworkPacketInfo.LocalAddressBroadcast

		// It's possible that a raw socket expects to receive this.
		pkt = nil

		// Take the base of the incoming request IP header but replace the options.
		replyHeaderLength := uint8(header.IPv4MinimumSize + len(newOptions))
		replyIPHdr := header.IPv4(append(iph[:header.IPv4MinimumSize:header.IPv4MinimumSize], newOptions...))
		replyIPHdr.SetHeaderLength(replyHeaderLength)

		// As per RFC 1122 section 3.2.1.3, when a host sends any datagram, the IP
		// source address MUST be one of its own IP addresses (but not a broadcast
		// or multicast address).
		localAddr := ipHdr.DestinationAddress()
		if localAddressBroadcast || header.IsV4MulticastAddress(localAddr) {
			localAddr = ""
		}

		r, err := nstack.FindRoute(1, localAddr, ipHdr.SourceAddress(), ipv4.ProtocolNumber, false /* multicastLoop */)
		if err != nil {
			// If we cannot find a route to the destination, silently drop the packet.
			return
		}
		defer r.Release()

		replyIPHdr.SetSourceAddress(r.LocalAddress())
		replyIPHdr.SetDestinationAddress(r.RemoteAddress())
		replyIPHdr.SetTTL(r.DefaultTTL())

		replyICMPHdr := header.ICMPv4(replyData)
		replyICMPHdr.SetType(header.ICMPv4EchoReply)
		replyICMPHdr.SetChecksum(0)
		replyICMPHdr.SetChecksum(^header.Checksum(replyData, 0))

		replyBuf := buffer.NewWithData(replyIPHdr)
		replyBuf.AppendOwned(replyData)
		replyPkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			ReserveHeaderBytes: int(r.MaxHeaderLength()),
			Payload:            replyBuf,
		})

		replyPkt.TransportProtocolNumber = header.ICMPv4ProtocolNumber

		if err := r.WriteHeaderIncludedPacket(replyPkt); err != nil {
			panic(err)
			return
		}
	}
}
