package client

import (
	"fmt"
	"io"
	"net"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func proxyChannel(user *users.User, newChannel ssh.NewChannel, log logger.Logger) {
	a := newChannel.ExtraData()

	var drtMsg internal.ChannelOpenDirectMsg
	err := ssh.Unmarshal(a, &drtMsg)
	if err != nil {
		log.Ulogf(logger.WARN, "Unable to unmarshal proxy %s\n", err)
		return
	}

	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Ulogf(logger.WARN, "Unable to accept new channel %s\n", err)
		return
	}
	go ssh.DiscardRequests(requests)

	tcpConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", drtMsg.Raddr, drtMsg.Rport))
	if err != nil {
		log.Ulogf(logger.WARN, "Unable to dial destination: %s\n", err)
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
