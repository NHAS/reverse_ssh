package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"sync"

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

			controllableClients = append(controllableClients, sshConn)

		} else {
			// Accept all channels
			go handleChannels(sshConn, chans)
		}

		// Discard all global out-of-band Requests
		go ssh.DiscardRequests(reqs)
	}
}

func handleChannels(sshConn ssh.Conn, chans <-chan ssh.NewChannel) {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		go handleChannel(sshConn, newChannel)
	}
}

func handleChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {
	// Since we're handling a shell, we expect a
	// channel type of "session". The also describes
	// "x11", "direct-tcpip" and "forwarded-tcpip"
	// channel types.
	if t := newChannel.ChannelType(); t != "session" {
		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
		log.Printf("Client %s (%s) sent invalid channel type '%s'\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), t)
		return
	}

	defer log.Printf("Client disconnected %s (%s)\n", sshConn.RemoteAddr(), sshConn.ClientVersion())

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}
	defer connection.Close()

	term := terminal.NewTerminal(connection, "> ")

	stop := make(chan bool)

	var ptyReq ssh.Request
	var lastWindowChange ssh.Request

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	go handleSSHRequests(&ptyReq, &lastWindowChange, term, requests, stop)

	fmt.Fprintf(term, "Connected controllable clients: \n")
	for i := range controllableClients {

		fmt.Fprintf(term, "%d. %s:%s\n",
			i,
			controllableClients[i].RemoteAddr(),
			controllableClients[i].ClientVersion(),
		)
	}

	i := -1
	var splice ssh.Channel
	for {
		//This will break if the user does CTRL+C or CTRL+D, not entirely sure why we cant just consume it. But whatever
		line, err := term.ReadLine()
		if err != nil {
			break
		}

		i, err = strconv.Atoi(line)
		if err != nil || i > len(controllableClients) || i < 0 {
			fmt.Fprintf(term, "Please enter a valid number\n")
			continue
		}

		splice, _, err = attachSession(i, ptyReq, lastWindowChange)
		close := func() { splice.Close(); stop <- true }
		if err == nil {
			stop <- true

			var once sync.Once
			go func() {
				io.Copy(connection, splice)
				once.Do(close)

			}()
			go func() {
				io.Copy(splice, connection)
				once.Do(close)

			}()

		RequestsPasser:
			for {
				select {
				case r := <-requests:
					response, err := sendRequest(*r, splice)
					if err != nil {
						fmt.Fprintf(term, "Error sending request: %s %s\n", r.Type, err)
						once.Do(func() { splice.Close() })
						break RequestsPasser
					}

					if r.WantReply {
						r.Reply(response, nil)
					}
				case <-stop:
					break RequestsPasser
				}

			}

			log.Printf("Client %s (%s) has disconnected from remote host %s (%s)\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), controllableClients[i].RemoteAddr(), controllableClients[i].ClientVersion())

			fmt.Fprintf(term, "Session has terminated\n")

			go handleSSHRequests(&ptyReq, &lastWindowChange, term, requests, stop)
			continue

		}

		fmt.Fprintf(term, err.Error())
	}
	stop <- true

}

func attachSession(i int, ptyReq, lastWindowChange ssh.Request) (sc ssh.Channel, r <-chan *ssh.Request, err error) {

	sshConn := controllableClients[i]

	splice, newrequests, err := sshConn.OpenChannel("session", nil)
	if err != nil {
		log.Printf("Unable to start remote session on host %s (%s) : %s\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), err)
		return sc, r, fmt.Errorf("Unable to start remote session on host %s (%s) : %s", sshConn.RemoteAddr(), sshConn.ClientVersion(), err)
	}

	//Replay the pty and any the very last window size change in order to correctly size the PTY on the controlled client
	_, err = sendRequest(ptyReq, splice)
	if err != nil {
		return sc, r, fmt.Errorf("Unable to send PTY request: %s", err)
	}

	_, err = sendRequest(lastWindowChange, splice)
	if err != nil {
		return sc, r, fmt.Errorf("Unable to send last window change request: %s", err)
	}

	go ssh.DiscardRequests(newrequests)

	return splice, newrequests, nil
}

func sendRequest(req ssh.Request, sshChan ssh.Channel) (bool, error) {
	return sshChan.SendRequest(req.Type, req.WantReply, req.Payload)
}

func handleSSHRequests(ptyr *ssh.Request, wc *ssh.Request, term *terminal.Terminal, requests <-chan *ssh.Request, cancel <-chan bool) {

	for {
		select {
		case <-cancel:
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
}
