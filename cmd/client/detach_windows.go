//go:build windows
// +build windows

package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/client"
)

func runOrFork(destination, proxyaddress, fingerprint string, fg, dt, rc bool) {
	if fg || dt {
		if dt {
			modkernel32 := syscall.NewLazyDLL("kernel32.dll")
			procAttachConsole := modkernel32.NewProc("FreeConsole")
			syscall.Syscall(procAttachConsole.Addr(), 0, 0, 0, 0)
		}
		client.Run(destination, fingerprint, proxyaddress, rc)
		return
	}

	cmd := exec.Command(os.Args[0], append([]string{"--detach"}, os.Args[1:]...)...)
	cmd.Start()
	log.Println("Ending parent")
}
