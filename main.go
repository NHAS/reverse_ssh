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
	"time"
)

func main() {
	if len(os.Args) > 1 {
		log.Println("Am client, forking to background and disowning parent")

		if len(os.Args) == 2 {
			cmd := exec.Command(os.Args[0], "--client", "fork")
			cmd.Start()

			<-time.Tick(time.Second * 20)
			log.Println("Ending parent")
			return
		}

		client()
		return
	}

	server()
}
