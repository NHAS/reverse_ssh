package server

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/handlers"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

var controllableClients sync.Map
var clientSysInfo map[string]string = make(map[string]string)
var autoCompleteClients *trie.Trie

func ReadPubKeys(path string) (m map[string]bool, err error) {
	authorizedKeysBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return m, fmt.Errorf("failed to load file %s, err: %v", path, err)
	}

	keys := bytes.Split(authorizedKeysBytes, []byte("\n"))
	m = map[string]bool{}

	for i, key := range keys {
		key = bytes.TrimSpace(key)
		if len(key) == 0 {
			continue
		}

		pubKey, _, _, _, err := ssh.ParseAuthorizedKey(key)
		if err != nil {
			return m, fmt.Errorf("unable to parse public key. %s line %d. Reason: %s", path, i+1, err)
		}

		m[string(pubKey.Marshal())] = true
	}

	return
}

func Run(addr, privateKeyPath string, insecure bool, publicKeyPath string) {

	//Taken from the server example, authorized keys are required for controllers
	log.Printf("Loading authorized keys from: %s\n", publicKeyPath)
	_, err := ReadPubKeys(publicKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = ReadPubKeys("proxy_keys")
	if err != nil {
		log.Println(err) // Not a fatal error, as you can just want *No* proxiers
	}

	_, err = ReadPubKeys("authorized_controllee_keys")
	if err != nil {
		if !insecure {
			log.Fatal(err)
		} else {
			log.Println(err)
		}
	}

	// In the latest version of crypto/ssh (after Go 1.3), the SSH server type has been removed
	// in favour of an SSH connection type. A ssh.ServerConn is created by passing an existing
	// net.Conn and a ssh.ServerConfig to ssh.NewServerConn, in effect, upgrading the net.Conn
	// into an ssh.ServerConn
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {

			authorizedKeysMap, err := ReadPubKeys(publicKeyPath)
			if err != nil {
				log.Println("Reloading authorized_keys failed: ", err)
			}

			authorizedProxiers, err := ReadPubKeys("proxy_keys")
			if err != nil && !strings.Contains(err.Error(), "Failed to load file") {
				log.Println(err)
			}

			authorizedControllees, err := ReadPubKeys("authorized_controllee_keys")
			if err != nil {
				log.Println("Reloading authorized_controllee_keys failed: ", err)
			}

			var clientType string

			//If insecure mode, then any unknown client will be connected as a controllable client.
			//The server effectively ignores channel requests from controllable clients.
			if authorizedKeysMap[string(key.Marshal())] {
				clientType = "user"
			} else if authorizedProxiers[string(key.Marshal())] {
				clientType = "proxy"
			} else if insecure || authorizedControllees[string(key.Marshal())] {
				clientType = "client"
			} else {
				return nil, fmt.Errorf("not authorized %q, potentially you might want to enabled -insecure mode", conn.User())
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

		go acceptConn(tcpConn, config)
	}

}

func acceptConn(tcpConn net.Conn, config *ssh.ServerConfig) {
	// Before use, a handshake must be performed on the incoming net.Conn.
	sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, config)
	if err != nil {
		log.Printf("Failed to handshake (%s)", err.Error())
		return
	}

	clientLog := logger.NewLog(sshConn.RemoteAddr().String())
	clientLog.Info("New SSH connection, version %s", sshConn.ClientVersion())

	switch sshConn.Permissions.Extensions["type"] {
	case "user":
		user, err := internal.AddUser(createIdString(sshConn), sshConn)
		if err != nil {
			sshConn.Close()
			log.Println(err)
			return
		}

		// Since we're handling a shell, local and remote forward, so we expect
		// channel type of "session" or "direct-tcpip", "forwarded-tcpip" respectively.
		go func() {
			internal.RegisterChannelCallbacks(user, chans, clientLog, map[string]internal.ChannelHandler{
				"session":      handlers.Session(&controllableClients, clientSysInfo, autoCompleteClients),
				"direct-tcpip": handlers.LocalForward,
			})

			internal.RemoveUser(user.IdString)
		}()

		// Discard all global out-of-band Requests, except for the tcpip-forward
		go func(in <-chan *ssh.Request) {
			for req := range in {

				switch req.Type {
				case "tcpip-forward":
					go handlers.RegisterRemoteForwardRequest(req, user)

				case "cancel-tcpip-forward":

					go func() {

						var rf internal.RemoteForwardRequest
						err = ssh.Unmarshal(req.Payload, &rf)
						if err != nil {
							req.Reply(false, nil)
							return
						}

						toClose := internal.RemoveFoward(rf, user)

						for _, id := range toClose {
							cc, ok := controllableClients.Load(id)
							if !ok {
								continue
							}

							clientConnection := cc.(ssh.Conn)

							clientConnection.SendRequest("cancel-tcpip-forward", true, ssh.Marshal(&rf))
						}

						clientLog.Info("Client just closed remote forwarding for %v", toClose)
					}()

				default:
					clientLog.Warning("Unhandled request %s", req.Type)
					if req.WantReply {
						req.Reply(false, nil)
					}

				}

			}
		}(reqs)
	case "client":
		idString := createIdString(sshConn)

		autoCompleteClients.Add(idString)

		controllableClients.Store(idString, sshConn)

		go internal.RegisterChannelCallbacks(nil, chans, clientLog, map[string]internal.ChannelHandler{
			"forwarded-tcpip": handlers.RemoteForward(idString),
		})

		go func(s string) {
			for req := range reqs {
				if req.Type == "sysinfo" {
					clientSysInfo[idString] = string(req.Payload)
				} else if req.WantReply {
					req.Reply(false, nil)
				}
			}

			clientLog.Info("SSH client disconnected")
			//Todo make less bad
			controllableClients.Delete(s)
			autoCompleteClients.Remove(s)
			internal.RemoveSource(s)
		}(idString)

	case "proxy":
		// Proxy is a special case, we dont want the client to have access to control elements, but want a port to be able to be opened.
		// I would have liked to wrap this into the callbacks register, however this has different requirements to the channel handlers.
		go internal.DiscardChannels(sshConn, chans, clientLog)
		//go handlers.RemoteForward(sshConn, reqs)

	default:
		sshConn.Close()
		clientLog.Warning("Client connected but type was unknown, terminating: ", sshConn.Permissions.Extensions["type"])
	}
}

func createIdString(sshServerConn *ssh.ServerConn) string {
	b := sha1.Sum([]byte(fmt.Sprintf("%s@%s", sshServerConn.Permissions.Extensions["pubkey-fp"], sshServerConn.RemoteAddr())))

	fingerPrint := hex.EncodeToString(b[:])
	return fingerPrint

}
