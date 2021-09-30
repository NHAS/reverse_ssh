package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NHAS/reverse_ssh/internal/server"
)

func printHelp() {

	fmt.Println("usage: ", filepath.Base(os.Args[0]), "[--key] [--publickeys] address")
	fmt.Println("\t\taddress\tThe network address the server will listen on")
	fmt.Println("\t\t--key\tPath to the ssh private key the server will use when talking with clients")
	fmt.Println("\t\t--authorizedkeys\tPath to the authorized_keys file or a given public key that control which users can talk to the server")

}

func main() {

	privkey_path := flag.String("key", "", "Path to SSH private key, if omitted will generate a key on first use")
	insecure := flag.Bool("insecure", false, "Ignore authorized_controllee_keys and allow any controllable client to connect")
	authkey_path := flag.String("authorizedkeys", "authorized_keys", "Path to authorized_keys file or a given public key, if omitted will look for an adjacent 'authorized_keys' file")

	flag.Usage = printHelp

	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("Missing listening address")
		printHelp()
		return
	}

	server.Run(flag.Args()[0], *privkey_path, *insecure, *authkey_path)

}
