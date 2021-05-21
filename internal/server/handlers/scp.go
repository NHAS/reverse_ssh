package handlers

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"golang.org/x/crypto/ssh"
)

func scp(connection ssh.Channel, requests <-chan *ssh.Request, mode string, path string, controllableClients *sync.Map) error {
	go ssh.DiscardRequests(requests)

	parts := strings.SplitN(path, ":", 2)

	if len(parts) < 1 {
		internal.ScpError("No target specified", connection)
		return nil
	}

	conn, ok := controllableClients.Load(parts[0])
	if !ok {
		internal.ScpError(fmt.Sprintf("Invalid target, %s not found", parts[0]), connection)
		return nil
	}

	device := conn.(ssh.Conn)

	scp, r, err := device.OpenChannel("scp", ssh.Marshal(&internal.Scp{Mode: mode, Path: parts[1]}))
	if err != nil {
		internal.ScpError("Could not connect to remote target", connection)
		return err
	}
	go ssh.DiscardRequests(r)

	go func() {
		defer scp.Close()
		defer connection.Close()

		io.Copy(connection, scp)
	}()

	defer scp.Close()
	defer connection.Close()
	io.Copy(scp, connection)

	return nil
}
