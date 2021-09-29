// go:build !windows
// +build !windows

package main

import (
	"log"
	"os"
	"os/exec"
)

func runInBackground(fingerprint, proxyAddress *string, rc bool) {
	cmd := exec.Command(os.Args[0], append([]string{"--foreground"}, os.Args[1:]...)...)
	cmd.Start()
	log.Println("Ending parent")
}
