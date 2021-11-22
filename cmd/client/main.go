package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/NHAS/reverse_ssh/internal/client/shellhost"
)

func printHelp() {

	fmt.Println("usage: ", filepath.Base(os.Args[0]), "[--foreground] [--fingerprint] destination")
	fmt.Println("\t\t--foreground\tCauses the client to run without forking to background")
	fmt.Println("\t\t--fingerprint\tServer public key SHA256 hex fingerprint for auth")
	fmt.Println("\t\t--reconnect\tReconnect on connection failure")
	fmt.Println("\t\t--proxy\tSets the HTTP_PROXY enviroment variable so the net library will use it")
}

var destination string

func main() {

	flag.Bool("foreground", false, "Dont fork to background on start")
	flag.Bool("no-reconnect", false, "Disable reconnect on disconnection")
	flag.Bool("detach", true, "(windows only) will force a console detach")
	var toExec *string
	if runtime.GOOS == "windows" {
		toExec = flag.String("exec", "powershell.exe", "")
	}

	proxyaddress := flag.String("proxy", "", "Sets the HTTP_PROXY enviroment variable so the net library will use it")
	fingerprint := flag.String("fingerprint", "", "Server public key fingerprint")

	flag.Usage = printHelp

	flag.Parse()

	var fg, dt, ex bool
	rc := true

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "exec":
			ex = true
		case "no-reconnect":
			rc = false
		case "foreground":
			fg = true
		case "detach":
			dt = true
		}
	})

	if ex && runtime.GOOS == "windows" {
		args := flag.Args()
		shellhost.ShellHost(*toExec, args...)
		return
	}

	if len(flag.Arg(0)) == 0 && len(destination) == 0 {
		fmt.Println("Missing destination (no default present)")
		printHelp()
		return
	}

	if len(flag.Arg(0)) != 0 {
		destination = flag.Arg(0)
	}

	runOrFork(destination, *proxyaddress, *fingerprint, fg, dt, rc)
}
