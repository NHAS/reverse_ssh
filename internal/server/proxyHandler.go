package server

import (
	"io"
	"log"
	"sync"

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

	log.Printf("Human client proxying to: %s:%d\n", drtMsg.Raddr, drtMsg.Rport)

	connection, requests, err := newChannel.Accept()
	defer connection.Close()
	go func() {
		for r := range requests {
			log.Println("Got req: ", r)
		}
	}()

	proxyDest, proxyRequests, err := destConn.OpenChannel(newChannel.ChannelType(), newChannel.ExtraData())
	defer proxyDest.Close()
	go func() {
		for r := range proxyRequests {
			log.Println("Prox Got req: ", r)
		}
	}()

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		io.Copy(connection, proxyDest)
		wg.Done()
	}()
	go func() {
		io.Copy(proxyDest, connection)
		wg.Done()
	}()

	wg.Wait()
}
