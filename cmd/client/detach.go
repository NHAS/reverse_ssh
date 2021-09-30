//go:build !windows
// +build !windows

package main

import (
	"log"
	"os"
	"os/exec"

	"github.com/NHAS/reverse_ssh/internal/client"
)

func runOrFork(destination, fingerprint, proxyaddress string, fg, dt, rc bool) {
	if fg {
		client.Run(destination, fingerprint, proxyaddress, rc)
		return
	}

	cmd := exec.Command(os.Args[0], append([]string{"--foreground"}, os.Args[1:]...)...)
	cmd.Start()
	log.Println("Ending parent")
}
