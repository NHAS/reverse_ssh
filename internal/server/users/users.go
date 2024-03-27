package users

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/data"
	"golang.org/x/crypto/ssh"
)

var ErrNilServerConnection = errors.New("the server connection was nil for the client")

var (
	lUsers sync.RWMutex
	// Username to actual user object
	users = map[string]*User{}

	activeConnections = map[string]bool{}
)

type Connection struct {
	// This is the users connection to the server itself, creates new channels and whatnot. NOT to get io.Copy'd
	serverConnection ssh.Conn

	Pty *internal.PtyReq

	ShellRequests <-chan *ssh.Request

	// So we can capture details about who is currently using the rssh server
	ConnectionDetails string
}

type User struct {
	sync.RWMutex

	userConnections map[string]*Connection
	username string

}

func (u *User) Session(connectionDetails string) (*Connection, error) {
	if c, ok := u.userConnections[connectionDetails]; ok {
		return c, nil
	}

	return nil, errors.New("session not found")
}

func (u *User) Username() string {
	return u.username
}

func (u *User) Privilege() int {
	priv, err := data.GetPrivilege(u.username)
	if err != nil {
		log.Println("was unable to get privs of", u.username, "defaulting to 0 (no priv)")
		return 0
	}

	return priv
}

func CreateOrGetUser(ServerConnection *ssh.ServerConn) (us *User, connectionDetails string, err error) {
	if ServerConnection == nil {
		err = ErrNilServerConnection
		return
	}

	lUsers.Lock()
	defer lUsers.Unlock()

	u, ok := users[ServerConnection.User()]
	if !ok {
		newUser := &User{
			username:        ServerConnection.User(),
			userConnections: map[string]*Connection{},
		}

		users[ServerConnection.User()] = newUser
		u = newUser
	}

	newConnection := &Connection{
		serverConnection:  ServerConnection,
		ShellRequests:     make(<-chan *ssh.Request),
		ConnectionDetails: makeConnectionDetailsString(ServerConnection),
	}

	if _, ok := u.userConnections[newConnection.ConnectionDetails]; ok {
		return nil, "", fmt.Errorf("connection already exists for %s", newConnection.ConnectionDetails)
	}

	u.userConnections[newConnection.ConnectionDetails] = newConnection
	activeConnections[newConnection.ConnectionDetails] = true

	return u, newConnection.ConnectionDetails, nil
}

func makeConnectionDetailsString(ServerConnection *ssh.ServerConn) string {
	return fmt.Sprintf("%s@%s", ServerConnection.User(), ServerConnection.RemoteAddr().String())
}

func ListUsers() (userList []string) {
	lUsers.RLock()
	defer lUsers.RUnlock()

	for user := range users {
		userList = append(userList, user)
	}

	sort.Strings(userList)
	return
}

func DisconnectUser(ServerConnection *ssh.ServerConn) {
	if ServerConnection != nil {
		lUsers.Lock()
		defer lUsers.Unlock()
		defer ServerConnection.Close()

		details := makeConnectionDetailsString(ServerConnection)

		user, ok := users[ServerConnection.User()]
		if !ok {
			return
		}

		delete(user.userConnections, details)
		delete(activeConnections, details)

		if len(user.userConnections) == 0 {
			delete(users, user.username)
		}
	}
}
