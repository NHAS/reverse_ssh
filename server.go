package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/NHAS/reverse_ssh/trie"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

var controllableClients []ssh.Conn

var autoCompleteTrie *trie.Trie

func server() {
	// In the latest version of crypto/ssh (after Go 1.3), the SSH server type has been removed
	// in favour of an SSH connection type. A ssh.ServerConn is created by passing an existing
	// net.Conn and a ssh.ServerConfig to ssh.NewServerConn, in effect, upgrading the net.Conn
	// into an ssh.ServerConn

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil // Temp shim
		},
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

	autoCompleteTrie = trie.NewTrie()
	autoCompleteTrie.Add("exit ")
	autoCompleteTrie.Add("ls ")
	autoCompleteTrie.Add("connect ")

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
		t := newChannel.ChannelType()
		switch t {
		case "session":
			go handleSessionChannel(sshConn, newChannel)
		case "direct-tcpip":
			go handleProxyChannel(sshConn, newChannel)
		default:
			newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
			log.Printf("Client %s (%s) sent invalid channel type '%s'\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), t)

		}

	}
}

type channelOpenDirectMsg struct {
	Raddr string
	Rport uint32
	Laddr string
	Lport uint32
}

func handleProxyChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {
	a := newChannel.ExtraData()

	var drtMsg channelOpenDirectMsg
	err := ssh.Unmarshal(a, &drtMsg)
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Printf("Client wanted to connect to: \n")
	fmt.Printf("L: %s:%d\n", drtMsg.Laddr, drtMsg.Lport)
	fmt.Printf("R: %s:%d\n", drtMsg.Raddr, drtMsg.Rport)

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

func handleSessionChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {
	// Since we're handling a shell, we expect a
	// channel type of "session". The also describes
	// "x11", "direct-tcpip" and "forwarded-tcpip"
	// channel types.

	defer log.Printf("Human client disconnected %s (%s)\n", sshConn.RemoteAddr(), sshConn.ClientVersion())

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}
	defer connection.Close()

	term := terminal.NewTerminal(connection, "> ")
	term.AutoCompleteCallback = func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {

		if key == '\t' {
			r := autoCompleteTrie.PrefixMatch(strings.TrimSpace(line))

			if len(r) == 1 {
				return line + r[0], len(line + r[0]), true
			}

			fmt.Fprintf(term, "%s\n", r)
			return "", 0, false
		}
		return "", 0, false
	}

	stop := make(chan bool)

	var ptyReq ssh.Request
	var lastWindowChange ssh.Request

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	// While we arent passing the requests directly to the remote host consume them with our terminal and store the results to send initialy to the remote on client connect
	go handleSSHRequests(&ptyReq, &lastWindowChange, term, requests, stop)
	defer func() {
		stop <- true // Stops the default handleSSHRequests as the channel gets closed which would cause a nil dereference
	}()

	//Send list of controllable remote hosts to human client
	fmt.Fprintf(term, "Connected controllable clients: \n")
	for i := range controllableClients {

		fmt.Fprintf(term, "%d. %s:%s\n",
			i,
			controllableClients[i].RemoteAddr(),
			controllableClients[i].ClientVersion(),
		)
	}

	for {
		//This will break if the user does CTRL+C or CTRL+D apparently we need to reset the whole terminal if a user does this....
		line, err := term.ReadLine()
		if err != nil {
			break
		}

		commandParts := strings.Split(line, " ")

		if len(commandParts) > 0 {

			switch commandParts[0] {
			default:
				fmt.Fprintf(term, "Unknown command: %s\n", commandParts[0])

			case "ls":
				for i := range controllableClients {

					fmt.Fprintf(term, "%d. %s:%s\n",
						i,
						controllableClients[i].RemoteAddr(),
						controllableClients[i].ClientVersion(),
					)
				}

			case "exit":
				return
			case "connect":
				if len(commandParts) != 2 {
					fmt.Fprintf(term, "connect <remote machine id>\n")
					continue
				}

				i, err := strconv.Atoi(commandParts[1])
				if err != nil || i > len(controllableClients) || i < 0 {
					fmt.Fprintf(term, "Please enter a valid number\n")
					continue
				}

				//Attempt to connect to remote host and send inital pty request and screen size
				// If we cant, report and error to the clients terminal
				newSession, err := createSession(i, ptyReq, lastWindowChange)
				if err == nil {
					stop <- true // Stop the default request handler
					err := attachSession(newSession, connection, requests)
					if err != nil {
						fmt.Fprintf(term, "Error: %s", err)
						log.Println(err)
					}
					fmt.Fprintf(term, "Session has terminated\n")
					log.Printf("Client %s (%s) has disconnected from remote host %s (%s)\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), controllableClients[i].RemoteAddr(), controllableClients[i].ClientVersion())

					go handleSSHRequests(&ptyReq, &lastWindowChange, term, requests, stop) // Re-enable the default handler if the client isnt connected to a remote host
				} else {

					fmt.Fprintf(term, "%s\n", err)
				}
			}
		}
	}

}

func createSession(i int, ptyReq, lastWindowChange ssh.Request) (sc ssh.Channel, err error) {

	sshConn := controllableClients[i]

	splice, newrequests, err := sshConn.OpenChannel("session", nil)
	if err != nil {
		log.Printf("Unable to start remote session on host %s (%s) : %s\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), err)
		return sc, fmt.Errorf("Unable to start remote session on host %s (%s) : %s", sshConn.RemoteAddr(), sshConn.ClientVersion(), err)
	}

	//Replay the pty and any the very last window size change in order to correctly size the PTY on the controlled client
	_, err = sendRequest(ptyReq, splice)
	if err != nil {
		return sc, fmt.Errorf("Unable to send PTY request: %s", err)
	}

	_, err = sendRequest(lastWindowChange, splice)
	if err != nil {
		return sc, fmt.Errorf("Unable to send last window change request: %s", err)
	}

	go ssh.DiscardRequests(newrequests)

	return splice, nil
}

func attachSession(newSession, currentClientSession ssh.Channel, currentClientRequests <-chan *ssh.Request) error {
	finished := make(chan bool)
	close := func() {
		newSession.Close()
		finished <- true // Stop the request passer on IO error
	}

	//Setup the pipes for stdin/stdout over the connections
	//Splice being the remote host being controlled
	var once sync.Once
	go func() {
		io.Copy(currentClientSession, newSession) // Potentially be more verbose about errors here
		once.Do(close)                            // Only close the splice connection once

	}()
	go func() {
		io.Copy(newSession, currentClientSession)
		once.Do(close)
	}()
	defer once.Do(close)

RequestsPasser:
	for {
		select {
		case r := <-currentClientRequests:
			response, err := sendRequest(*r, newSession)
			if err != nil {
				break RequestsPasser
			}

			if r.WantReply {
				r.Reply(response, nil)
			}
		case <-finished:
			break RequestsPasser
		}

	}

	return nil
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
