package client

import (
	"fmt"
	"io"
	"net"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func proxyChannel(user *internal.User, newChannel ssh.NewChannel, l logger.Logger) {
	a := newChannel.ExtraData()

	var drtMsg internal.ChannelOpenDirectMsg
	err := ssh.Unmarshal(a, &drtMsg)
	if err != nil {
		l.Warning("Unable to unmarshal proxy %s", err)
		return
	}

	connection, requests, err := newChannel.Accept()
	if err != nil {
		l.Warning("Unable to accept new channel %s", err)
		return
	}
	go ssh.DiscardRequests(requests)

	tcpConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", drtMsg.Raddr, drtMsg.Rport))
	if err != nil {
		l.Warning("Unable to dial destination: %s", err)
		return
	}

	go func() {
		defer tcpConn.Close()
		defer connection.Close()

		io.Copy(connection, tcpConn)

	}()
	go func() {
		defer tcpConn.Close()
		defer connection.Close()

		io.Copy(tcpConn, connection)
	}()
}
