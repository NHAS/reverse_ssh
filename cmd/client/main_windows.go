//go:build windows
// +build windows

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/client"
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
	flag.Bool("reconnect", true, "Auto reconnect on disconnection")
	flag.Bool("detach", true, "(windows only) will force a console detach")
	proxyAddress := flag.String("proxy", "", "Sets the HTTP_PROXY enviroment variable so the net library will use it")
	fingerprint := flag.String("fingerprint", "", "Server public key fingerprint")

	flag.Usage = printHelp

	flag.Parse()

	var fg, rc, dt bool

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "reconnect":
			rc = true
		case "foreground":
			fg = true
		case "detach":
			dt = true
		}
	})

	if len(flag.Arg(0)) == 0 && len(destination) == 0 {
		fmt.Println("Missing destination (no default present)")
		printHelp()
		return
	}

	if len(flag.Arg(0)) != 0 {
		destination = flag.Arg(0)
	}

	if fg || dt {
		if dt {
			modkernel32 := syscall.NewLazyDLL("kernel32.dll")
			procAttachConsole := modkernel32.NewProc("FreeConsole")
			syscall.Syscall(procAttachConsole.Addr(), 0, 0, 0, 0)
		}
		client.Run(destination, *fingerprint, *proxyAddress, rc)
		return
	}

	newArgs := append([]string{"--foreground"}, os.Args[1:]...)
	if runtime.GOOS == "windows" {
		newArgs[0] = "--detach" // will trigger a detach
	}
	cmd := exec.Command(os.Args[0], newArgs...)
	cmd.Start()
	log.Println("Ending parent")
}
