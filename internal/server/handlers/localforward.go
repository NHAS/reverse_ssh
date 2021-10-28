package handlers

import (
	"io"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/clients"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func LocalForward(user *internal.User, newChannel ssh.NewChannel, log logger.Logger) {
	proxyTarget := newChannel.ExtraData()

	var drtMsg internal.ChannelOpenDirectMsg
	err := ssh.Unmarshal(proxyTarget, &drtMsg)
	if err != nil {
		log.Warning("Unable to unmarshal proxy destination: %s", err)
		return
	}
	log.Info("Before")
	target, err := clients.Get(drtMsg.Raddr)
	if err != nil {
		newChannel.Reject(ssh.Prohibited, err.Error())
		return
	}
	log.Info("After")

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

	go io.Copy(connection, targetConnection)
	io.Copy(targetConnection, connection)

}
