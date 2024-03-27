package connection

import (
	"fmt"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type Session struct {
	sync.RWMutex

	// This is the users connection to the server itself, creates new channels and whatnot. NOT to get io.Copy'd
	ServerConnection ssh.Conn

	Pty *internal.PtyReq

	ShellRequests <-chan *ssh.Request

	// Remote forwards sent by user, used to just close user specific remote forwards
	SupportedRemoteForwards map[internal.RemoteForwardRequest]bool //(set)
}

func NewSession(connection ssh.Conn) *Session {

	return &Session{
		ServerConnection:        connection,
		SupportedRemoteForwards: make(map[internal.RemoteForwardRequest]bool),
	}
}

func RegisterChannelCallbacks(chans <-chan ssh.NewChannel, log logger.Logger, handlers map[string]func(newChannel ssh.NewChannel, log logger.Logger)) error {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		t := newChannel.ChannelType()
		log.Info("Handling channel: %s", t)
		if callBack, ok := handlers[t]; ok {
			go callBack(newChannel, log)
			continue
		}

		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unsupported channel type: %s", t))
		log.Warning("Sent an invalid channel type %q", t)
	}

	return fmt.Errorf("connection terminated")
}
