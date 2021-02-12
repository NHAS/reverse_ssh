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
		client()
	default:
		cmd := exec.Command(os.Args[0], "--client")
		cmd.Start()
		log.Println("Ending parent")
	}

}
