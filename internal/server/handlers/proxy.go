package handlers

import (
	"io"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func Proxy(user *users.User, newChannel ssh.NewChannel, log logger.Logger) {

	if user.ProxyConnection == nil {
		newChannel.Reject(ssh.Prohibited, "no remote location to forward traffic to")
		return
	}

	proxyTarget := newChannel.ExtraData()

	var drtMsg internal.ChannelOpenDirectMsg
	err := ssh.Unmarshal(proxyTarget, &drtMsg)
	if err != nil {
		log.Ulogf(logger.WARN, "Unable to unmarshal proxy destination: %s\n", err)
		return
	}

	connection, requests, err := newChannel.Accept()
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, err.Error())
		return
	}

	go ssh.DiscardRequests(requests)

	proxyDest, proxyRequests, err := user.ProxyConnection.OpenChannel(newChannel.ChannelType(), newChannel.ExtraData())
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, err.Error())
		return
	}

	log.Logf("Human client proxying to: %s:%d\n", drtMsg.Raddr, drtMsg.Rport)

	go ssh.DiscardRequests(proxyRequests)

	go func() {
		defer proxyDest.Close()
		defer connection.Close()

		io.Copy(connection, proxyDest)
	}()
	go func() {
		defer proxyDest.Close()
		defer connection.Close()
		io.Copy(proxyDest, connection)

		log.Logf("ENDED: %s:%d\n", drtMsg.Raddr, drtMsg.Rport)

	}()

}
