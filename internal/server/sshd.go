package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"time"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/clients"
	"github.com/NHAS/reverse_ssh/internal/server/handlers"
	"github.com/NHAS/reverse_ssh/internal/server/observers"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

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

func StartSSHServer(sshListener net.Listener, privateKey ssh.Signer, insecure bool, authorizedKeys string) {
	//Taken from the server example, authorized keys are required for controllers
	log.Printf("Loading authorized keys from: %s\n", authorizedKeys)
	authorizedControllers, err := readPubKeys(authorizedKeys)
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
		ServerVersion: "SSH-2.0-OpenSSH_7.4",
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {

			authorizedKeysMap, err := readPubKeys(authorizedKeys)
			if err != nil {
				log.Println("Reloading authorized_keys failed: ", err)
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

	config.AddHostKey(privateKey)

	// Accept all connections

	for {
		tcpConn, err := sshListener.Accept()
		if err != nil {
			log.Printf("Failed to accept incoming connection (%s)", err)
			continue
		}

		go acceptConn(tcpConn, config)
	}
}

func acceptConn(c net.Conn, config *ssh.ServerConfig) {

	realConn := &internal.TimeoutConn{c, 10 * time.Second}

	// Before use, a handshake must be performed on the incoming net.Conn.
	sshConn, chans, reqs, err := ssh.NewServerConn(realConn, config)
	if err != nil {
		log.Printf("Failed to handshake (%s)", err.Error())
		return
	}

	clientLog := logger.NewLog(sshConn.RemoteAddr().String())

	go func() {
		for {
			_, _, err = sshConn.SendRequest("keepalive@golang.org", true, nil)
			if err != nil {
				clientLog.Info("Failed to send keepalive, assuming client has disconnected")
				sshConn.Close()
				return
			}
			time.Sleep(5 * time.Second)
		}
	}()

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

		id, username, err := clients.Add(sshConn)
		if err != nil {
			clientLog.Error("Unable to add new client %s", err)

			sshConn.Close()
			return
		}

		go func() {
			ssh.DiscardRequests(reqs)

			clientLog.Info("SSH client disconnected")
			clients.Remove(id)
			observers.ConnectionState.Notify(observers.ClientState{
				Status:    "disconnected",
				ID:        id,
				IP:        sshConn.RemoteAddr().String(),
				HostName:  username,
				Timestamp: time.Now(),
			})
		}()

		clientLog.Info("New controllable connection with id %s", id)

		observers.ConnectionState.Notify(observers.ClientState{
			Status:    "connected",
			ID:        id,
			IP:        sshConn.RemoteAddr().String(),
			HostName:  username,
			Timestamp: time.Now(),
		})

	default:
		sshConn.Close()
		clientLog.Warning("Client connected but type was unknown, terminating: %s", sshConn.Permissions.Extensions["type"])
	}
}
