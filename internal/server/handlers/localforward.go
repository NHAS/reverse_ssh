package handlers

import (
	"fmt"
	"io"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func LocalForward(controllableClients *sync.Map) internal.ChannelHandler {

	return func(user *internal.User, newChannel ssh.NewChannel, log logger.Logger) {
		proxyTarget := newChannel.ExtraData()

		var drtMsg internal.ChannelOpenDirectMsg
		err := ssh.Unmarshal(proxyTarget, &drtMsg)
		if err != nil {
			log.Warning("Unable to unmarshal proxy destination: %s", err)
			return
		}

		c, ok := controllableClients.Load(drtMsg.Raddr)
		if !ok {
			newChannel.Reject(ssh.Prohibited, fmt.Sprintf("Target %s not found, use ls to list clients", drtMsg.Raddr))
			return
		}

		target := c.(ssh.Conn)

		targetConnection, targetRequests, err := target.OpenChannel("jump", nil)
		if err != nil {
			newChannel.Reject(ssh.ConnectionFailed, err.Error())
			return
		}
		defer targetConnection.Close()
		go ssh.DiscardRequests(targetRequests)

		connection, requests, err := newChannel.Accept()
		if err != nil {
			newChannel.Reject(ssh.ConnectionFailed, err.Error())
			return
		}
		defer connection.Close()
		go ssh.DiscardRequests(requests)

		go io.Copy(targetConnection, connection)
		io.Copy(connection, targetConnection)

	}
}
