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

	Pty *PtyReq

	// Remote forwards sent by user
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
