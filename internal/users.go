package internal

import (
	"errors"

	"golang.org/x/crypto/ssh"
)

var ErrNilServerConnection = errors.New("The server connection was nil for the client")

type User struct {
	// This is the users connection to the server itself, creates new channels and whatnot. NOT to get io.Copy'd
	ServerConnection ssh.Conn

	Pty *PtyReq

	// Remote forwards sent by user
	SupportedRemoteForwards map[RemoteForwardRequest]bool //(set)
}

func CreateUser(ServerConnection ssh.Conn) (us *User, err error) {
	if ServerConnection == nil {
		err = ErrNilServerConnection
		return
	}

	us = &User{
		ServerConnection:        ServerConnection,
		SupportedRemoteForwards: make(map[RemoteForwardRequest]bool),
	}

	return
}

func DeleteUser(us *User) {
	if us != nil {
		if us.ServerConnection != nil {
			defer us.ServerConnection.Close()
		}
	}
}
