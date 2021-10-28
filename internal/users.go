package internal

import (
	"errors"
	"sync"

	"golang.org/x/crypto/ssh"
)

var lock sync.RWMutex
var allUsers = make(map[string]*User)

var ErrNilServerConnection = errors.New("The server connection was nil for the client")

type User struct {
	IdString string
	// This is the users connection to the server itself, creates new channels and whatnot. NOT to get io.Copy'd
	ServerConnection ssh.Conn

	//What the client input is currently being sent to
	ShellRequests <-chan *ssh.Request

	ProxyConnection ssh.Conn

	Pty *PtyReq

	EnabledRcfiles map[string][]string

	// Remote forwards sent by user
	// As these may collide with another users requests (as they come in the form of 1234:localhost:1234)
	// We store them per user, waiting for the user to tell us what client they want to start the remote forward itself
	// with the exec handler
	SupportedRemoteForwards map[RemoteForwardRequest]bool //(set)
}

func AddUser(ServerConnection ssh.Conn) (us *User, err error) {
	lock.Lock()
	defer lock.Unlock()

	if ServerConnection == nil {
		err = ErrNilServerConnection
		return
	}

	idStr, err := RandomString(20)
	if err != nil {
		return nil, err
	}

	us = &User{
		IdString:                idStr,
		ServerConnection:        ServerConnection,
		EnabledRcfiles:          make(map[string][]string),
		SupportedRemoteForwards: make(map[RemoteForwardRequest]bool),
	}

	allUsers[idStr] = us

	return
}

func RemoveUser(idStr string) {
	lock.Lock()
	defer lock.Unlock()

	if us, ok := allUsers[idStr]; ok {
		// Do not close down the proxy connection, as this is the remote controlled hosts connection to here.
		// This would terminate other users connecting to the controllable host
		if us.ServerConnection != nil {
			defer us.ServerConnection.Close()
		}

	}

	delete(allUsers, idStr)

}
