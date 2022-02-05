package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/clients"
	"github.com/NHAS/reverse_ssh/internal/server/handlers"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func CreateOrLoadServerKeys(privateKeyPath string) (ssh.Signer, error) {
	if privateKeyPath == "" {
		//If we have already created a private key (or there is one in the current directory) dont overwrite/create another one
		privateKeyPath = "id_ed25519"
		if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {

			privateKeyPem, err := internal.GeneratePrivateKey()
			if err != nil {
				return nil, fmt.Errorf("Unable to generate private key, and no private key specified: %s", err)
			}

			err = ioutil.WriteFile(privateKeyPath, privateKeyPem, 0600)
			if err != nil {
				return nil, fmt.Errorf("Unable to write private key to disk: %s", err)
			}
		}

	}

	privateBytes, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to load private key (%s): %s", privateKeyPath, err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse private key: %s", err)
	}

	return private, nil
}

func readPubKeys(path string) (m map[string]bool, err error) {
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
	authorizedControllers, err := readPubKeys(publicKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	clients, err := readPubKeys("authorized_controllee_keys")
	if err != nil {
		if !insecure {
			log.Fatal(err)
		} else {
			log.Println(err)
		}
	}

	for key := range clients {
		if _, ok := authorizedControllers[key]; ok {
			log.Fatalf("[ERROR] Key %s is present in both authorized_controllee_keys and authorized_keys. It should only be in one.", key)
		}
	}

	// In the latest version of crypto/ssh (after Go 1.3), the SSH server type has been removed
	// in favour of an SSH connection type. A ssh.ServerConn is created by passing an existing
	// net.Conn and a ssh.ServerConfig to ssh.NewServerConn, in effect, upgrading the net.Conn
	// into an ssh.ServerConn
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {

			authorizedKeysMap, err := readPubKeys(publicKeyPath)
			if err != nil {
				log.Println("Reloading authorized_keys failed: ", err)
			}

			authorizedProxiers, err := readPubKeys("proxy_keys")
			if err != nil && !strings.Contains(err.Error(), "Failed to load file") {
				log.Println(err)
			}

			authorizedControllees, err := readPubKeys("authorized_controllee_keys")
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

	private, err := CreateOrLoadServerKeys(privateKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	s, err := filepath.Abs(privateKeyPath)
	if err != nil {
		log.Fatalf("Unable to make absolute path from private key path [%s]: %s", privateKeyPath, err)
	}

	log.Printf("Loading private key from: %s (%s)\n", privateKeyPath, s)
	log.Println("Server key fingerprint: ", internal.FingerprintSHA256Hex(private.PublicKey()))

	config.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be accepted.
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s (%s)", addr, err)
	}

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

	switch sshConn.Permissions.Extensions["type"] {
	case "user":
		user, err := internal.CreateUser(sshConn)
		if err != nil {
			sshConn.Close()
			log.Println(err)
			return
		}

		// Since we're handling a shell, local and remote forward, so we expect
		// channel type of "session" or "direct-tcpip", "forwarded-tcpip" respectively.
		go func() {
			err = internal.RegisterChannelCallbacks(user, chans, clientLog, map[string]internal.ChannelHandler{
				"session":      handlers.Session,
				"direct-tcpip": handlers.LocalForward,
			})
			clientLog.Info("User disconnected: %s", err.Error())

			internal.DeleteUser(user)
		}()

		clientLog.Info("New User SSH connection, version %s", sshConn.ClientVersion())

		// Discard all global out-of-band Requests, except for the tcpip-forward
		go ssh.DiscardRequests(reqs)

	case "client":

		id, err := clients.Add(sshConn)
		if err != nil {
			clientLog.Error("Unable to add new client %s", err)

			sshConn.Close()
			return
		}

		go func() {
			ssh.DiscardRequests(reqs)

			clientLog.Info("SSH client disconnected")
			clients.Remove(id)
		}()

		clientLog.Info("New controllable connection")

	default:
		sshConn.Close()
		clientLog.Warning("Client connected but type was unknown, terminating: ", sshConn.Permissions.Extensions["type"])
	}
}
