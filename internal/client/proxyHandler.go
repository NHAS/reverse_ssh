package client

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/NHAS/reverse_ssh/internal"
	"golang.org/x/crypto/ssh"
)

func proxyChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {
	a := newChannel.ExtraData()

	var drtMsg internal.ChannelOpenDirectMsg
	err := ssh.Unmarshal(a, &drtMsg)
	if err != nil {
		log.Println(err)
		return
	}

	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Println("Unable to accept new channel", err)
		return
	}
	go ssh.DiscardRequests(requests)

	tcpConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", drtMsg.Raddr, drtMsg.Rport))
	if err != nil {
		log.Println(err)
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
