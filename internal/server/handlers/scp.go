package handlers

import (
	"sync"

	"golang.org/x/crypto/ssh"
)

func scp(connection ssh.Channel, requests <-chan *ssh.Request, mode string, path string, controllableClients *sync.Map) error {
	go ssh.DiscardRequests(requests)

	//log.Println("Mode: ", mode, "Path: ", path)

	return nil
}
