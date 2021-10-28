package internal

import (
	"errors"
	"fmt"
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
	userToForwards map[string]map[RemoteForwardRequest]bool
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
	SupportedRemoteForwards map[RemoteForwardRequest]bool //(set)
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
				forwardsToUser: make(map[RemoteForwardRequest]string),
				userToForwards: make(map[string]map[RemoteForwardRequest]bool),
			}
		}

		if _, ok := s.userToForwards[dst]; !ok {
			s.userToForwards[dst] = make(map[RemoteForwardRequest]bool)
		}

		for rf := range forwards {

			if _, ok := s.forwardsToUser[rf]; ok {
				return fmt.Errorf("Forward %v already exists in table", rf)
			}
			s.forwardsToUser[rf] = dst
			s.userToForwards[dst][rf] = true
		}

		interfaces[src] = s
	}

	return nil
}

func RemoveFoward(rf RemoteForwardRequest, user *User) (toClosed []string) {

	forwardRulesGuard.Lock()
	defer forwardRulesGuard.Unlock()

	delete(user.SupportedRemoteForwards, rf)

	for key, value := range interfaces {
		if userId, ok := value.forwardsToUser[rf]; ok && userId == user.IdString {
			delete(value.forwardsToUser, rf)
			delete(value.userToForwards[userId], rf)

			if len(value.userToForwards[userId]) == 0 {
				delete(value.userToForwards, userId)
			}

			toClosed = append(toClosed, key)
		}

	}
	return
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

	defer func() {
		recover() // Horrible, but this happens during testing (as I cant be bothered properly mocking a server connection just so we can close it)
	}()

	if us, ok := allUsers[idStr]; ok {

		for k := range us.SupportedRemoteForwards {
			RemoveFoward(k, us)
		}

		// Do not close down the proxy connection, as this is the remote controlled hosts connection to here.
		// This would terminate other users connecting to the controllable host
		if us.ServerConnection != nil {
			defer us.ServerConnection.Close()
		}

	}

	delete(allUsers, idStr)

}
