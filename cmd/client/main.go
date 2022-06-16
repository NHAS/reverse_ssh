package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/terminal"
)

var destination string
var fingerprint string
var ignoreInput string

func printHelp() {
	fmt.Println("usage: ", filepath.Base(os.Args[0]), "[--foreground] [--fingerprint] destination")
	fmt.Println("\t\t--foreground\tCauses the client to run without forking to background")
	fmt.Println("\t\t--fingerprint\tServer public key SHA256 hex fingerprint for auth")
	fmt.Println("\t\t--proxy\tLocation of HTTP connect proxy to use")
}

func main() {

	if len(os.Args) == 0 || ignoreInput == "true" {
		Run(destination, fingerprint, "")
		return
	}

	var argv = strings.Join(os.Args, " ")

	//If we have a fd, it is a pipe which we can read options from and act if we are forked
	o := os.NewFile(uintptr(3), "pipe")
	orginialArgv, err := io.ReadAll(o)
	child := false
	if err == nil && len(orginialArgv) > 0 {
		argv = string(orginialArgv)
		child = true
	}
	o.Close()

	line := terminal.ParseLine(argv, 0)

	if line.IsSet("h") || line.IsSet("help") {
		printHelp()
		return
	}

	if len(line.Arguments) < 1 && len(destination) == 0 {
		fmt.Println("No destination specified")
		printHelp()
		return
	}

	if len(line.Arguments) > 0 {
		destination = line.Arguments[len(line.Arguments)-1].Value()
	}

	fg := line.IsSet("foreground") || child

	proxyaddress, _ := line.GetArgString("proxy")

	userSpecifiedFingerprint, err := line.GetArgString("fingerprint")
	if err == nil {
		fingerprint = userSpecifiedFingerprint
	}

	if fg {
		Run(destination, fingerprint, proxyaddress)
		return
	}

	err = Fork(destination, fingerprint, proxyaddress)
	if err != nil {
		log.Fatal("Unable to fork: ", err)
		Run(destination, fingerprint, proxyaddress)
	}

}
