package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/commands/constants"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type connect struct {
	log                 logger.Logger
	user                *internal.User
	defaultHandle       *WindowSizeChangeHandler
	controllableClients *sync.Map
	term                *terminal.Terminal

	init     func()
	teardown func()
}

func (c *connect) Run(tty io.ReadWriter, args ...string) error {
	if len(args) != 1 {
		return fmt.Errorf(c.Help(false))
	}

	cc, ok := c.controllableClients.Load(args[0])
	if !ok {
		return fmt.Errorf("Unknown connection host")
	}

	controlClient := cc.(ssh.Conn)

	defer func() {

		c.log.Info("Disconnected from remote host %s (%s)", controlClient.RemoteAddr(), controlClient.ClientVersion())

		c.defaultHandle.Start() // Re-enable the default handler if the client isnt connected to a remote host

	}()

	//Attempt to connect to remote host and send inital pty request and screen size
	// If we cant, report and error to the clients terminal
	newSession, err := createSession(controlClient, *c.user.Pty)
	if err != nil {

		c.log.Error("Creating session failed: %s", err)
		return err
	}

	c.defaultHandle.Stop()

	c.log.Info("Connected to %s", controlClient.RemoteAddr().String())

	if c.init != nil {
		c.init()
	}

	if c.teardown != nil {
		defer c.teardown()
	}
	err = attachSession(newSession, tty, c.user.ShellRequests, c.user.EnabledRcfiles[args[0]])
	if err != nil {

		c.log.Error("Client tried to attach session and failed: %s", err)
		return err
	}

	return fmt.Errorf("Session has terminated.") // Not really an error. But we can get the terminal to print out stuff

}

func (c *connect) Expect(sections []string) []string {

	if len(sections) == 1 {
		return []string{constants.RemoteId}
	}

	return nil
}

func (c *connect) Help(explain bool) string {
	if explain {
		return "Start shell on remote controllable host."
	}

	return makeHelpText(
		"connect <remote_id>",
	)
}

func Connect(
	user *internal.User,
	controllableClients *sync.Map,
	defaultHandle *WindowSizeChangeHandler,
	log logger.Logger,
	initFunc func(),
	teardownFunc func()) *connect {
	return &connect{
		user:                user,
		defaultHandle:       defaultHandle,
		controllableClients: controllableClients,
		log:                 log,
		init:                initFunc,
		teardown:            teardownFunc,
	}
}

func createSession(sshConn ssh.Conn, ptyReq internal.PtyReq) (sc ssh.Channel, err error) {

	splice, newrequests, err := sshConn.OpenChannel("session", nil)
	if err != nil {
		return sc, fmt.Errorf("Unable to start remote session on host %s (%s) : %s", sshConn.RemoteAddr(), sshConn.ClientVersion(), err)
	}

	//Send pty request, pty has been continuously updated with window-change sizes
	_, err = splice.SendRequest("pty-req", true, ssh.Marshal(ptyReq))
	if err != nil {
		return sc, fmt.Errorf("Unable to send PTY request: %s", err)
	}

	go ssh.DiscardRequests(newrequests)

	return splice, nil
}

func attachSession(newSession ssh.Channel, currentClientSession io.ReadWriter, currentClientRequests <-chan *ssh.Request, rcfiles []string) error {

	finished := make(chan bool)

	close := func() {
		newSession.Close()
		finished <- true // Stop the request passer on IO error
	}

	//Setup the pipes for stdin/stdout over the connections

	//Start copying output before we start copying user input, so we can get the responses to the rc files lines
	var once sync.Once
	defer once.Do(close)

	go func() {
		//dst <- src
		io.Copy(newSession, currentClientSession)
		once.Do(close)

	}()

	for _, path := range rcfiles {
		file, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(currentClientSession, "Unable to open rc file: %s\n", path)
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			_, err := newSession.Write([]byte(scanner.Text() + "\n"))
			if err != nil {
				return fmt.Errorf("Unable to read lines from rc file %s : %s", path, err)
			}
		}
	}

	//newSession being the remote host being controlled
	go func() {
		io.Copy(currentClientSession, newSession) // Potentially be more verbose about errors here
		once.Do(close)                            // Only close the newSession connection once

	}()

RequestsProxyPasser:
	for {
		select {
		case r := <-currentClientRequests:
			response, err := internal.SendRequest(*r, newSession)
			if err != nil {
				break RequestsProxyPasser
			}

			if r.WantReply {
				r.Reply(response, nil)
			}
		case <-finished:
			break RequestsProxyPasser
		}

	}

	return nil
}
