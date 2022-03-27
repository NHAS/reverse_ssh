package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/terminal"
)

var destination string
var fingerprint string

func printHelp() {

	fmt.Println("usage: ", filepath.Base(os.Args[0]), "[--foreground] [--fingerprint] destination")
	fmt.Println("\t\t--foreground\tCauses the client to run without forking to background")
	fmt.Println("\t\t--fingerprint\tServer public key SHA256 hex fingerprint for auth")
	fmt.Println("\t\t--no-reconnect\tDisable reconnect on connection failure")
	fmt.Println("\t\t--proxy\tSets the HTTP_PROXY enviroment variable so the net library will use it")
}

func main() {

	//Happens if we're executing from a fileless state
	if len(os.Args) == 0 {
		runOrFork(destination, fingerprint, "", false, true, true)
		return
	}

	line := terminal.ParseLine(strings.Join(os.Args, " "), 0)

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

	fg := line.IsSet("foreground")
	dt := line.IsSet("detach")

	proxyaddress, _ := line.GetArgString("proxy")

	userSpecifiedFingerprint, err := line.GetArgString("fingerprint")
	if err == nil {
		fingerprint = userSpecifiedFingerprint
	}

	rc := !line.IsSet("no-reconnect")

	runOrFork(destination, fingerprint, proxyaddress, fg, dt, rc)

}
