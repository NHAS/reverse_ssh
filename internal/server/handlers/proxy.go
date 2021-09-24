package handlers

import (
	"io"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func Proxy(user *internal.User, newChannel ssh.NewChannel, log logger.Logger) {

	if user.ProxyConnection == nil {
		newChannel.Reject(ssh.Prohibited, "no remote location to forward traffic to")
		return
	}

	proxyTarget := newChannel.ExtraData()

	var drtMsg internal.ChannelOpenDirectMsg
	err := ssh.Unmarshal(proxyTarget, &drtMsg)
	if err != nil {
		log.Warning("Unable to unmarshal proxy destination: %s", err)
		return
	}

	proxyDest, proxyRequests, err := user.ProxyConnection.OpenChannel(newChannel.ChannelType(), newChannel.ExtraData())
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	defer proxyDest.Close()

	connection, requests, err := newChannel.Accept()
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	defer connection.Close()

	go ssh.DiscardRequests(requests)

	log.Info("Human client proxying to: %s:%d", drtMsg.Raddr, drtMsg.Rport)

	go ssh.DiscardRequests(proxyRequests)

	go func() {
		defer proxyDest.Close()
		defer connection.Close()

		io.Copy(connection, proxyDest)
	}()

	io.Copy(proxyDest, connection)

	log.Info("ENDED: %s:%d", drtMsg.Raddr, drtMsg.Rport)

}
