package server

import (
	"io"
	"log"

	"github.com/NHAS/reverse_ssh/internal"
	"golang.org/x/crypto/ssh"
)

func proxyChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {

	if connections[sshConn] == nil {
		newChannel.Reject(ssh.Prohibited, "no remote location to forward traffic to")
		return
	}

	destConn := connections[sshConn]

	proxyTarget := newChannel.ExtraData()

	var drtMsg internal.ChannelOpenDirectMsg
	err := ssh.Unmarshal(proxyTarget, &drtMsg)
	if err != nil {
		log.Println(err)
		return
	}

	connection, requests, err := newChannel.Accept()
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, err.Error())
		return
	}

	go ssh.DiscardRequests(requests)

	proxyDest, proxyRequests, err := destConn.OpenChannel(newChannel.ChannelType(), newChannel.ExtraData())
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, err.Error())
		return
	}

	log.Printf("Human client proxying to: %s:%d\n", drtMsg.Raddr, drtMsg.Rport)

	go ssh.DiscardRequests(proxyRequests)

	go func() {
		defer proxyDest.Close()
		defer connection.Close()

		io.Copy(connection, proxyDest)
	}()
	go func() {
		defer proxyDest.Close()
		defer connection.Close()
		io.Copy(proxyDest, connection)

		log.Printf("ENDED: %s:%d\n", drtMsg.Raddr, drtMsg.Rport)

	}()

}
