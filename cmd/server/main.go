package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NHAS/reverse_ssh/internal/server"
)

func printHelp() {

	fmt.Println("usage: ", filepath.Base(os.Args[0]), "[--key] address")
	fmt.Println("\t\taddress\tThe network address the server will listen on")
	fmt.Println("\t\t--key\tPath to the ssh private key the server will use")

}

func main() {

	privkey_path := flag.String("key", "", "Path to SSH private key, if omitted will generate a key on first use")
	insecure := flag.Bool("insecure", false, "Ignore authorized_controllee_keys and allow any controllable client to connect")

	flag.Usage = printHelp

	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("Missing listening address")
		printHelp()
		return
	}

	server.Run(flag.Args()[0], *privkey_path, *insecure)

}
