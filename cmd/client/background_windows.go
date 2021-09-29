//go:build windows
// +build windows

package main

import (
	"log"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/client"
)

func runInBackground(fingerprint, proxyAddress *string, rc bool) {
	log.Println("Ending parent")
	modkernel32 := syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole := modkernel32.NewProc("FreeConsole")
	syscall.Syscall(procAttachConsole.Addr(), 0, 0, 0, 0)

	client.Run(destination, *fingerprint, *proxyAddress, rc)
}
