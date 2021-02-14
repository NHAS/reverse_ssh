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
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	arg := ""

	if len(os.Args) > 1 {
		arg = strings.TrimSpace(os.Args[1])
	}

	switch arg {
	case "--server":
		server()
	case "--client":
		serverKey := ""
		if len(os.Args) == 3 {
			serverKey = os.Args[2]
		}
		client(serverKey)
	default:
		cmd := exec.Command(os.Args[0], append([]string{"--client"}, os.Args[1:]...)...)
		cmd.Start()
		log.Println("Ending parent")
	}

}
