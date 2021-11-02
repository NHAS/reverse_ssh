package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/NHAS/reverse_ssh/internal/server"
)

func printHelp() {

	fmt.Println("usage: ", filepath.Base(os.Args[0]), "[--key] [--authorizedkeys] address")
	fmt.Println("\t\taddress\tThe network address the server will listen on")
	fmt.Println("\t\t--key\tPath to the ssh private key the server will use when talking with clients")
	fmt.Println("\t\t--authorizedkeys\tPath to the authorized_keys file or a given public key that control which users can talk to the server")
	fmt.Println("\t\t--insecure\tIgnore authorized_controllee_keys and allow any controllable client to connect")
	fmt.Println("\t\t--daemonise\tFork to background")

}

func main() {

	privkey_path := flag.String("key", "", "Path to SSH private key, if omitted will generate a key on first use")
	flag.Bool("insecure", false, "Ignore authorized_controllee_keys and allow any controllable client to connect")
	flag.Bool("daemonise", false, "Go to background")
	authkey_path := flag.String("authorizedkeys", "authorized_keys", "Path to authorized_keys file or a given public key, if omitted will look for an adjacent 'authorized_keys' file")

	flag.Usage = printHelp

	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("Missing listening address")
		printHelp()
		return
	}

	var background, insecure bool

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "insecure":
			insecure = true
		case "daemonise":
			background = true
		}
	})

	if background {
		cmd := exec.Command(os.Args[0], flag.Args()...)
		cmd.Start()
		log.Println("Ending parent")
		return
	}

	server.Run(flag.Args()[0], *privkey_path, insecure, *authkey_path)

}
