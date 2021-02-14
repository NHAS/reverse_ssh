package client

import (
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/kr/pty"
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
	defer connection.Close()
	go func() {
		for r := range requests {
			log.Println("Got req: ", r)
		}
	}()

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

//This basically handles exactly like a SSH server would
func shellChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}

	// Fire up bash for this session
	bash := exec.Command("bash")

	// Prepare teardown function
	close := func() {
		connection.Close() // Not a fan of this
		_, err := bash.Process.Wait()
		if err != nil {
			log.Printf("Failed to exit bash (%s)", err)
		}
		log.Printf("Session closed")
	}

	// Allocate a terminal for this channel
	log.Print("Creating pty...")
	bashf, err := pty.Start(bash)
	if err != nil {
		log.Printf("Could not start pty (%s)", err)
		close()
		return
	}

	//pipe session to bash and visa-versa
	var once sync.Once
	go func() {
		io.Copy(connection, bashf)
		once.Do(close)
	}()
	go func() {
		io.Copy(bashf, connection)
		once.Do(close)
	}()

	for req := range requests {
		log.Println("Got request: ", req.Type)
		switch req.Type {
		case "shell":
			// We only accept the default shell
			// (i.e. no command in the Payload)
			if len(req.Payload) == 0 {
				req.Reply(true, nil)
			}
		case "pty-req":
			termLen := req.Payload[3]
			w, h := internal.ParseDims(req.Payload[termLen+4:])
			internal.SetWinsize(bashf.Fd(), w, h)
			// Responding true (OK) here will let the client
			// know we have a pty ready for input
			req.Reply(true, nil)
		case "window-change":
			w, h := internal.ParseDims(req.Payload)
			internal.SetWinsize(bashf.Fd(), w, h)
		}
	}

}
