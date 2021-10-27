package handlers

import (
	"fmt"
	"io"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func RegisterRemoteForwardRequest(r *ssh.Request, user *internal.User) {
	var rf internal.RemoteForwardRequest

	err := ssh.Unmarshal(r.Payload, &rf)
	if err != nil {
		r.Reply(false, []byte(fmt.Sprintf("Unable to open remote forward: %s", err.Error())))
		return
	}

	if rf.BindPort == 0 {
		r.Reply(false, []byte("Binding to next avaliable port is not supported sorry!"))
		return
	}

	user.SupportedRemoteForwards[rf] = true

	r.Reply(true, nil)
}

// Enable remote forwarding to rssh clients
func RemoteForward(id string) internal.ChannelHandler {
	return func(user *internal.User, newChannel ssh.NewChannel, log logger.Logger) {
		// Reduce the chance of someone using the user object (despite it always being nil here)
		remoteForward(id, newChannel, log)
	}
}

func remoteForward(targetId string, newChannel ssh.NewChannel, log logger.Logger) {
	remotePorts := newChannel.ExtraData()

	var drtMsg internal.ChannelOpenDirectMsg
	err := ssh.Unmarshal(remotePorts, &drtMsg)
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, "Unable to unmarshal data")
		return
	}

	dest, err := internal.GetDestination(targetId, internal.RemoteForwardRequest{BindAddr: drtMsg.Raddr, BindPort: drtMsg.Rport})
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, err.Error())
		return
	}

	newChan, reqs, err := dest.OpenChannel(newChannel.ChannelType(), remotePorts)
	if err != nil {
		newChannel.Reject(ssh.ResourceShortage, err.Error())
		return
	}

	go ssh.DiscardRequests(reqs)

	connection, requests, err := newChannel.Accept()
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	defer connection.Close()

	log.Info("Remote Forward: %s:%d", drtMsg.Raddr, drtMsg.Rport)

	go ssh.DiscardRequests(requests)

	go func() {
		defer newChan.Close()
		defer connection.Close()

		io.Copy(connection, newChan)
	}()

	io.Copy(newChan, connection)

	log.Info("ENDED: %s:%d", drtMsg.Raddr, drtMsg.Rport)

}
