package server

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/handlers"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

var controllableClients sync.Map
var autoCompleteCommands, autoCompleteClients *trie.Trie

func ReadPubKeys(path string) (m map[string]bool, err error) {
	authorizedKeysBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return m, fmt.Errorf("Failed to load %s, err: %v", path, err)
	}

	m = map[string]bool{}
	for len(authorizedKeysBytes) > 0 {
		pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			return m, err
		}

		m[string(pubKey.Marshal())] = true
		authorizedKeysBytes = rest
	}

	return
}

func Run(addr, privateKeyPath string) {

	//Taken from the server example, authorized keys are required for controllers
	authorizedKeysMap, err := ReadPubKeys("authorized_keys")
	if err != nil {
		log.Fatal(err)
	}

	authorizedProxiers, err := ReadPubKeys("proxy_keys")
	if err != nil {
		log.Println(err) // Not a fatal error, as you can just want *No* proxiers
	}

	// In the latest version of crypto/ssh (after Go 1.3), the SSH server type has been removed
	// in favour of an SSH connection type. A ssh.ServerConn is created by passing an existing
	// net.Conn and a ssh.ServerConfig to ssh.NewServerConn, in effect, upgrading the net.Conn
	// into an ssh.ServerConn
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			var clientType string

			if conn.User() == "0d87be75162ded36626cb97b0f5b5ef170465533" {
				clientType = "slave"
			} else if authorizedKeysMap[string(key.Marshal())] {
				clientType = "master"
			} else if authorizedProxiers[string(key.Marshal())] {
				clientType = "proxy"
			} else {
				return nil, fmt.Errorf("Not authorized %q", conn.User())
			}

			return &ssh.Permissions{
				// Record the public key used for authentication.
				Extensions: map[string]string{
					"pubkey-fp": internal.FingerprintSHA1Hex(key),
					"type":      clientType,
				},
			}, nil

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

	log.Println("Server key fingerprint: ", internal.FingerprintSHA1Hex(private.PublicKey()))

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

		switch sshConn.Permissions.Extensions["type"] {
		case "master":
			user, err := users.AddUser(createIdString(sshConn), sshConn)
			if err != nil {
				sshConn.Close()
				log.Println(err)
				continue
			}

			// Since we're handling a shell and dynamic forward, so we expect
			// channel type of "session" or "direct-tcpip".
			go internal.RegisterChannelCallbacks(user, chans, map[string]internal.ChannelHandler{
				"session":      handlers.Session(&controllableClients, autoCompleteClients),
				"direct-tcpip": handlers.Proxy,
			})

			// Discard all global out-of-band Requests
			go ssh.DiscardRequests(reqs)
		case "slave":
			idString := createIdString(sshConn)

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

		case "proxy":
			// Proxy is a special case, we dont want the client to have access to control elements, but want a port to be able to be opened.
			// I would have liked to wrap this into the callbacks register, however this has different requirements to the channel handlers.
			go internal.DiscardChannels(sshConn, chans)
			go handlers.RemoteForward(sshConn, reqs)

		default:
			sshConn.Close()
			log.Println("Client connected but type was unknown, terminating: ", sshConn.Permissions.Extensions["type"])
		}

	}

}

func createIdString(sshServerConn *ssh.ServerConn) string {
	b := sha1.Sum([]byte(fmt.Sprintf("%s@%s", sshServerConn.Permissions.Extensions["pubkey-fp"], sshServerConn.RemoteAddr())))

	fingerPrint := hex.EncodeToString(b[:])
	return fingerPrint

}
