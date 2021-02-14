// A small SSH daemon providing bash sessions
//
// Server:
// cd my/new/dir/
// #generate server keypair
// ssh-keygen -t rsa
// go get -v .
// go run sshd.go
//
// Client:
// ssh foo@localhost -p 2200 #pass=bar

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func printHelp() {
	fmt.Println("Reverse SSH")
	fmt.Println(os.Args[0], " [--server | --client] [server ip] [server public key hex]")
	fmt.Println("Flags:")
	fmt.Println("\t--client\t\tStarts the client in foreground mode")
	fmt.Println("\t--server\t\tStarts the server listening on 0.0.0.0:2200")
	fmt.Println("Options:")
	fmt.Println("\t[server ip]\t\tIf in client mode, connect to this IP:port, if in server mode listen on this address")
	fmt.Println("\t[server public key hex]\t\tA hex sha256 fingerprint of the servers key, only used by the client to validate server")
	fmt.Println("Default:")
	fmt.Println("If started without '--client' the client will fork to background and connect to localhost:2200")
}

func main() {
	arg := ""

	if len(os.Args) > 1 {
		arg = strings.TrimSpace(os.Args[1])
	}

	serverIP := "localhost:2200"
	if len(os.Args) > 2 {
		serverIP = os.Args[2]
	}

	switch arg {
	case "-h", "-help", "--help":
		printHelp()
	case "--server":
		server(serverIP)
	case "--client":
		serverKey := ""
		if len(os.Args) == 4 {
			serverKey = os.Args[3]
		}
		client(serverIP, serverKey)
	default:
		cmd := exec.Command(os.Args[0], append([]string{"--client"}, os.Args[1:]...)...)
		cmd.Start()
		log.Println("Ending parent")
	}

}
