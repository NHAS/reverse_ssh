package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

var controllableClients sync.Map

//A map of 'controller' ssh connections, to the host they're controlling.
//Will be nil if they arent connected to anything
var connections map[ssh.Conn]ssh.Conn = make(map[ssh.Conn]ssh.Conn)

var autoCompleteCommands, autoCompleteClients *trie.Trie

func Run(addr, privateKeyPath string) {

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

	if privateKeyPath == "" {
		//If we have already created a private key (or there is one in the current directory) dont overwrite/create another one
		privateKeyPath = "id_ed25519"
		if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {

			privateKeyPem, err := internal.GeneratePrivateKey()
			if err != nil {
				log.Fatalf("Unable to generate private key, and no private key specified: %s", err)
			}

			err = ioutil.WriteFile(privateKeyPath, privateKeyPem, 0600)
			if err != nil {
				log.Fatalf("Unable to write private key to disk: %s", err)
			}

			log.Println("Auto generated new private key")
		}

	}

	s, err := filepath.Abs(privateKeyPath)
	if err != nil {
		log.Fatalf("Unable to make absolute path from private key path [%s]: %s", privateKeyPath, err)
	}

	log.Printf("Loading private key from: %s (%s)\n", privateKeyPath, s)

	privateBytes, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		log.Fatalf("Failed to load private key (%s): %s", privateKeyPath, err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatalf("Failed to parse private key: %s", err)
	}

	log.Println("Server key fingerprint: ", internal.FingerprintSHA256Hex(private.PublicKey()))

	config.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be accepted.
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s (%s)", addr, err)
	}

	autoCompleteClients = trie.NewTrie()

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

			autoCompleteClients.Add(idString)

			controllableClients.Store(idString, sshConn)

			go func(s string) {
				for req := range reqs {
					if req.WantReply {
						req.Reply(false, nil)
					}
				}
				log.Printf("SSH client disconnected %s", s)
				controllableClients.Delete(s)
				autoCompleteClients.Remove(idString)
			}(idString)

		} else {
			connections[sshConn] = nil

			// Since we're handling a shell and dynamic forward, so we expect
			// channel type of "session" or "direct-tcpip".
			go internal.RegisterChannelCallbacks(sshConn, chans, map[string]internal.ChannelHandler{
				"session":      sessionChannel,
				"direct-tcpip": proxyChannel,
			})

			// Discard all global out-of-band Requests
			go ssh.DiscardRequests(reqs)
		}

	}

}