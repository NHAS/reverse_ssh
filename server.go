package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/NHAS/reverse_ssh/trie"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

var controllableClients map[string]ssh.Conn = make(map[string]ssh.Conn)

//A map of 'controller' ssh connections, to the host they're controlling.
//Will be nil if they arent connected to anything
var connections map[ssh.Conn]ssh.Conn = make(map[ssh.Conn]ssh.Conn)

var autoCompleteTrie *trie.Trie

func server() {

	//Taken from the server example, authorized keys are required for controllers
	authorizedKeysBytes, err := ioutil.ReadFile("authorized_keys")
	if err != nil {
		log.Fatalf("Failed to load authorized_keys, err: %v", err)
	}

	authorizedKeysMap := map[string]bool{}
	for len(authorizedKeysBytes) > 0 {
		pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			log.Fatal(err)
		}

		authorizedKeysMap[string(pubKey.Marshal())] = true
		authorizedKeysBytes = rest
	}

	// In the latest version of crypto/ssh (after Go 1.3), the SSH server type has been removed
	// in favour of an SSH connection type. A ssh.ServerConn is created by passing an existing
	// net.Conn and a ssh.ServerConfig to ssh.NewServerConn, in effect, upgrading the net.Conn
	// into an ssh.ServerConn
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			controllable := "no"
			if conn.User() == "0d87be75162ded36626cb97b0f5b5ef170465533" {
				controllable = "yes"
			}

			if authorizedKeysMap[string(key.Marshal())] || controllable == "yes" {
				return &ssh.Permissions{
					// Record the public key used for authentication.
					Extensions: map[string]string{
						"pubkey-fp":    FingerprintSHA256Hex(key),
						"controllable": controllable,
					},
				}, nil
			}
			return nil, fmt.Errorf("unknown public key for %q", conn.User())
		},
	}

	// You can generate a keypair with 'ssh-keygen -t ed25519'
	privateBytes, err := ioutil.ReadFile("key")
	if err != nil {
		log.Fatal("Failed to load private key (./key)")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key")
	}

	log.Println("Server key fingerprint: ", FingerprintSHA256Hex(private.PublicKey()))

	config.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be accepted.
	listener, err := net.Listen("tcp", "0.0.0.0:2200")
	if err != nil {
		log.Fatalf("Failed to listen on 2200 (%s)", err)
	}

	autoCompleteTrie = trie.NewTrie()
	autoCompleteTrie.Add("exit")
	autoCompleteTrie.Add("ls")
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

		if sshConn.Permissions.Extensions["controllable"] == "yes" {

			idString := fmt.Sprintf("%s@%s", sshConn.Permissions.Extensions["pubkey-fp"], sshConn.RemoteAddr())

			autoCompleteTrie.Add(idString)

			controllableClients[idString] = sshConn

			go func(s string) {
				for req := range reqs {
					if req.WantReply {
						req.Reply(false, nil)
					}
				}
				log.Printf("SSH client disconnected %s", s)
				delete(controllableClients, s) // So so so not threadsafe, need to fix this
				autoCompleteTrie.Remove(idString)
			}(idString)

		} else {
			connections[sshConn] = nil

			// Since we're handling a shell and proxy, so we expect
			// channel type of "session" or "direct-tcpip".
			go handleChannels(sshConn, chans, map[string]channelHandler{
				"session":      handleSessionChannel,
				"direct-tcpip": handleProxyChannel,
			})

			// Discard all global out-of-band Requests
			go ssh.DiscardRequests(reqs)
		}

	}

}

func handleProxyChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {

	if connections[sshConn] == nil {
		newChannel.Reject(ssh.Prohibited, "no remote location to forward traffic to")
		return
	}

	destConn := connections[sshConn]

	proxyTarget := newChannel.ExtraData()

	var drtMsg channelOpenDirectMsg
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

func handleSessionChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {

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

			parts := strings.Split(strings.TrimSpace(line), " ")

			searchString := ""
			if len(parts) > 0 {
				searchString = parts[len(parts)-1]
			}

			r := autoCompleteTrie.PrefixMatch(searchString)

			if len(r) == 1 {
				return line + r[0], len(line + r[0]), true
			}

			for _, completion := range r {
				fmt.Fprintf(term, "%s\n", line+completion)
			}

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
	for idStr := range controllableClients {

		fmt.Fprintf(term, "%s, client version: %s\n",
			idStr,
			controllableClients[idStr].ClientVersion(),
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
				for idStr := range controllableClients {

					fmt.Fprintf(term, "%s, client version: %s\n",
						idStr,
						controllableClients[idStr].ClientVersion(),
					)
				}

			case "exit":
				return
			case "connect":
				if len(commandParts) != 2 {
					fmt.Fprintf(term, "connect <remote machine id>\n")
					continue
				}

				controlClient, ok := controllableClients[commandParts[1]]
				if !ok {
					fmt.Fprintf(term, "Unknown connection host\n")
					continue
				}
				//Attempt to connect to remote host and send inital pty request and screen size
				// If we cant, report and error to the clients terminal
				newSession, err := createSession(controlClient, ptyReq, lastWindowChange)
				if err == nil {
					stop <- true // Stop the default request handler

					connections[sshConn] = controlClient

					err := attachSession(newSession, connection, requests)
					if err != nil {
						fmt.Fprintf(term, "Error: %s", err)
						log.Println(err)
					}

					connections[sshConn] = nil

					fmt.Fprintf(term, "Session has terminated\n")
					log.Printf("Client %s (%s) has disconnected from remote host %s (%s)\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), controlClient.RemoteAddr(), controlClient.ClientVersion())

					go handleSSHRequests(&ptyReq, &lastWindowChange, term, requests, stop) // Re-enable the default handler if the client isnt connected to a remote host
				} else {

					fmt.Fprintf(term, "%s\n", err)
				}
			}
		}
	}

	delete(connections, sshConn)
}

func createSession(sshConn ssh.Conn, ptyReq, lastWindowChange ssh.Request) (sc ssh.Channel, err error) {

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
	//newSession being the remote host being controlled
	var once sync.Once
	go func() {
		io.Copy(currentClientSession, newSession) // Potentially be more verbose about errors here
		once.Do(close)                            // Only close the newSession connection once

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
				req.Reply(len(req.Payload) == 0, nil)
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
