//go:build !windows
// +build !windows

package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/client"
)

func runOrFork(destination, fingerprint, proxyaddress string, fg, dt, rc bool) {
	if fg {

		if os.Getuid() != 0 {
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
					if os.Geteuid() > int(i.Uid) {
						syscall.Setuid(int(i.Uid))
					}

					if os.Getegid() > int(i.Gid) {
						syscall.Setgid(int(i.Gid))
					}
				}
			}
		}

		client.Run(destination, fingerprint, proxyaddress, rc)
		return
	}

	cmd := exec.Command(os.Args[0], append([]string{"--foreground"}, os.Args[1:]...)...)
	cmd.Start()
	cmd.Process.Release()
	log.Println("Forking")
}
