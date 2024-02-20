package connection

import (
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
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
