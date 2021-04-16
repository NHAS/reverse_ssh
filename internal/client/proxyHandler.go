package client

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"

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
	defer connection.Close()
	go ssh.DiscardRequests(requests)

	tcpConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", drtMsg.Raddr, drtMsg.Rport))
	if err != nil {
		log.Println(err)
		return
	}
	defer tcpConn.Close()

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		io.Copy(connection, tcpConn)
		wg.Done()
	}()
	go func() {
		io.Copy(tcpConn, connection)
		wg.Done()
	}()

	wg.Wait()
}
