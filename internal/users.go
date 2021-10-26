package internal

import (
	"errors"
	"sync"

	"golang.org/x/crypto/ssh"
)

var lock sync.RWMutex
var allUsers = make(map[string]*User)

var ErrNilServerConnection = errors.New("The server connection was nil for the client")

var forwardRulesGuard sync.RWMutex
var interfaces = make(map[string]interfaceRules)

type interfaceRules struct {
	forwardsToUser map[RemoteForwardRequest]string
	userToForwards map[string][]RemoteForwardRequest
}

type User struct {
	IdString string
	// This is the users connection to the server itself, creates new channels and whatnot. NOT to get io.Copy'd
	ServerConnection ssh.Conn

	//What the client input is currently being sent to
	ShellRequests <-chan *ssh.Request

	ProxyConnection ssh.Conn

	Pty *PtyReq

	EnabledRcfiles map[string][]string

	SupportedRemoteForwards []RemoteForwardRequest
}

func EnableForwarding(dst string, sources ...string) error {
	forwardRulesGuard.Lock()
	defer forwardRulesGuard.Unlock()

	u := allUsers[dst]
	for _, rf := range u.SupportedRemoteForwards {

		rules, ok := userToForwards[userIdStr]
		if !ok {
			userToForwards[userIdStr] = append(userToForwards[userIdStr], rf)
			return nil
		}

		if _, ok := forwardsToUser[rf]; ok {
			return errors.New("Forward already exists in table")
		}

		forwardsToUser[rf] = userIdStr

		for _, v := range rules {
			if rf == v {
				return nil // Already exists
			}
		}

		userToForwards[userIdStr] = append(userToForwards[userIdStr], rf)
	}

	return nil
}

func GetDestination(target string, rf RemoteForwardRequest) (ssh.Conn, error) {
	forwardRulesGuard.RLock()
	defer forwardRulesGuard.RUnlock()

	ir, ok := interfaces[target]
	if !ok {
		return nil, errors.New("Could not find target: " + target)
	}

	idStr, ok := ir.forwardsToUser[rf]
	if !ok {
		return nil, errors.New("Forward is not associated with any user")
	}

	u, ok := allUsers[idStr]
	if !ok {
		return nil, errors.New("User id string was not found in all users table")
	}

	return u.ServerConnection, nil
}

func AddUser(idStr string, ServerConnection ssh.Conn) (us *User, err error) {
	lock.Lock()
	defer lock.Unlock()

	if ServerConnection == nil {
		err = ErrNilServerConnection
		return
	}

	us = &User{
		IdString:         idStr,
		ServerConnection: ServerConnection,
		EnabledRcfiles:   make(map[string][]string),
	}

	allUsers[idStr] = us

	return
}

func RemoveUser(idStr string) {
	lock.Lock()
	defer lock.Unlock()

	forwardRulesGuard.Lock()
	defer forwardRulesGuard.Unlock()

	if us, ok := allUsers[idStr]; ok {
		// Do not close down the proxy connection, as this is the remote controlled hosts connection to here.
		// This would terminate other users connecting to the controllable host

		if us.ServerConnection != nil {
			us.ServerConnection.Close()
		}

	}

	delete(allUsers, idStr)

	if rf, ok := userToForwards[idStr]; ok {
		for _, v := range rf {
			delete(forwardsToUser, v)
		}
		delete(userToForwards, idStr)
	}
}
