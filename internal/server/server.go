package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

var controllableClients map[string]ssh.Conn = make(map[string]ssh.Conn)

//A map of 'controller' ssh connections, to the host they're controlling.
//Will be nil if they arent connected to anything
var connections map[ssh.Conn]ssh.Conn = make(map[ssh.Conn]ssh.Conn)
var autoCompleteTrie *trie.Trie

func Run(addr string) {

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
						"pubkey-fp":    internal.FingerprintSHA256Hex(key),
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

	log.Println("Server key fingerprint: ", internal.FingerprintSHA256Hex(private.PublicKey()))

	config.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be accepted.
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on 2200 (%s)", err)
	}

	autoCompleteTrie = trie.NewTrie()
	autoCompleteTrie.Add("exit")
	autoCompleteTrie.Add("ls")
	autoCompleteTrie.Add("connect ")

	// Accept all connections
	log.Printf("Listening on %s...\n", addr)
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
			go internal.RegisterChannelCallbacks(sshConn, chans, map[string]internal.ChannelHandler{
				"session":      handleSessionChannel,
				"direct-tcpip": handleProxyChannel,
			})

			// Discard all global out-of-band Requests
			go ssh.DiscardRequests(reqs)
		}

	}

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
				w, h := internal.ParseDims(req.Payload[termLen+4:])
				term.SetSize(int(w), int(h))
				// Responding true (OK) here will let the client
				// know we have a pty ready for input
				req.Reply(true, nil)
				*ptyr = *req
			case "window-change":
				w, h := internal.ParseDims(req.Payload)
				term.SetSize(int(w), int(h))

				*wc = *req
			}
		}

	}
}
