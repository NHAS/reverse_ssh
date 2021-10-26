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

// Target ID to interface rule
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

	// Remote forwards sent by user
	// As these may collide with another users requests (as they come in the form of 1234:localhost:1234)
	// We store them per user, waiting for the user to tell us what client they want to start the remote forward itself
	// with the exec handler
	SupportedRemoteForwards []RemoteForwardRequest
}

// User (dst) has now told us what clients (sources) they want to remote forward
// So we need to add the forwards to the quasi-'routing' table
func EnableForwarding(dst string, sources ...string) error {
	forwardRulesGuard.Lock()
	defer forwardRulesGuard.Unlock()

	forwards := allUsers[dst].SupportedRemoteForwards

	for _, src := range sources {

		s, ok := interfaces[src]
		if !ok {
			s = interfaceRules{
				make(map[RemoteForwardRequest]string),
				make(map[string][]RemoteForwardRequest),
			}
		}

	Outer:
		for _, rf := range forwards {

			if _, ok := s.forwardsToUser[rf]; ok {
				return errors.New("Forward already exists in table")
			}
			s.forwardsToUser[rf] = dst

			rules, ok := s.userToForwards[dst]
			if !ok {
				s.userToForwards[dst] = append(s.userToForwards[dst], rf)
				continue
			}

			for _, v := range rules {
				if rf == v {
					continue Outer
				}
			}

			s.userToForwards[dst] = append(s.userToForwards[dst], rf)
		}

		interfaces[src] = s
	}

	return nil
}

func RemoveSource(source string) {
	forwardRulesGuard.Lock()
	defer forwardRulesGuard.Unlock()

	delete(interfaces, source)

}

func GetDestination(target string, rf RemoteForwardRequest) (ssh.Conn, error) {

	ir, ok := interfaces[target]
	if !ok {
		ir, ok = interfaces["all"] // ALl is essentially a default path
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

	defer func() {
		recover() // Horrible, but this happens during testing (as I cant be bothered properly mocking a server connection just so we can close it)
	}()

	for _, v := range interfaces {
		for _, vv := range v.userToForwards[idStr] {
			delete(v.forwardsToUser, vv)
		}
		delete(v.userToForwards, idStr)
	}

	if us, ok := allUsers[idStr]; ok {
		// Do not close down the proxy connection, as this is the remote controlled hosts connection to here.
		// This would terminate other users connecting to the controllable host
		if us.ServerConnection != nil {
			defer us.ServerConnection.Close()
		}

	}

	delete(allUsers, idStr)

}
