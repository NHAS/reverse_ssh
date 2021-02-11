package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strconv"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

var controllableClients []ssh.Conn

func server() {
	// In the latest version of crypto/ssh (after Go 1.3), the SSH server type has been removed
	// in favour of an SSH connection type. A ssh.ServerConn is created by passing an existing
	// net.Conn and a ssh.ServerConfig to ssh.NewServerConn, in effect, upgrading the net.Conn
	// into an ssh.ServerConn

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil // Temp shim
		},
		// You may also explicitly allow anonymous client authentication, though anon bash
		// sessions may not be a wise idea
		// NoClientAuth: true,
	}

	// You can generate a keypair with 'ssh-keygen -t rsa'
	privateBytes, err := ioutil.ReadFile("key")
	if err != nil {
		log.Fatal("Failed to load private key (./key)")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key")
	}

	config.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be accepted.
	listener, err := net.Listen("tcp", "0.0.0.0:2200")
	if err != nil {
		log.Fatalf("Failed to listen on 2200 (%s)", err)
	}

	// Accept all connections
	log.Print("Listening on 2200...")
	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept incoming connection (%s)", err)
			continue
		}
		// Before use, a handshake must be performed on the incoming net.Conn.
		sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, config)
		if err != nil {
			log.Printf("Failed to handshake (%s)", err)
			continue
		}
		log.Printf("New SSH connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())

		answer, _, _ := sshConn.SendRequest("reverse?", true, nil)
		if answer {
			log.Println("It wants the reverse!")
			controllableClients = append(controllableClients, sshConn)
		} else {
			// Accept all channels
			go handleChannels(chans)
		}

		// Discard all global out-of-band Requests
		go ssh.DiscardRequests(reqs)
	}
}

func handleChannels(chans <-chan ssh.NewChannel) {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		go handleChannel(newChannel)
	}
}

func handleChannel(newChannel ssh.NewChannel) {
	// Since we're handling a shell, we expect a
	// channel type of "session". The also describes
	// "x11", "direct-tcpip" and "forwarded-tcpip"
	// channel types.

	log.Println("Handling channel request: ", newChannel.ChannelType())
	if t := newChannel.ChannelType(); t != "session" {
		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
		return
	}

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}
	defer connection.Close()

	term := terminal.NewTerminal(connection, "> ")

	stop := make(chan string)
	var ptyReq ssh.Request
	var lastWindowChange ssh.Request

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	go func(ptyr *ssh.Request, wc *ssh.Request, term *terminal.Terminal) {

		for {
			select {
			case <-stop:
				return
			case req := <-requests:
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
					w, h := parseDims(req.Payload[termLen+4:])
					term.SetSize(int(w), int(h))
					// Responding true (OK) here will let the client
					// know we have a pty ready for input
					req.Reply(true, nil)
					*ptyr = *req
				case "window-change":
					w, h := parseDims(req.Payload)
					term.SetSize(int(w), int(h))
					*wc = *req
				}
			}

		}
	}(&ptyReq, &lastWindowChange, term)

	term.Write([]byte("Connected controllable clients: \n"))
	for i := range controllableClients {

		term.Write([]byte(
			fmt.Sprintf("%d. %s:%s\n",
				i,
				controllableClients[i].RemoteAddr(),
				controllableClients[i].ClientVersion(),
			)))
	}

	i := -1

	for {
		line, err := term.ReadLine()
		if err != nil {
			break
		}

		i, err = strconv.Atoi(line)
		if err != nil {
			term.Write([]byte("Please enter a valid number"))
			continue
		}

		break

	}

	splice, newrequests, err := controllableClients[i].OpenChannel("session", nil)
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}

	stop <- "now"

	sendRequest(ptyReq, splice)
	sendRequest(lastWindowChange, splice)

	go func() {
		io.Copy(connection, splice)

	}()
	go func() {
		io.Copy(splice, connection)

	}()

	go ssh.DiscardRequests(newrequests)

	for r := range requests {
		sendRequest(*r, splice)
	}

	log.Println("Client disconnected")

}

func sendRequest(req ssh.Request, sshChan ssh.Channel) (bool, error) {
	return sshChan.SendRequest(req.Type, req.WantReply, req.Payload)
}
