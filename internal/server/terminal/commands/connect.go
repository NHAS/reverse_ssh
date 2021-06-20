package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal/commands/constants"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type connect struct {
	log                 logger.Logger
	user                *users.User
	defaultHandle       *internal.DefaultSSHHandler
	controllableClients *sync.Map
}

func (c *connect) Run(term *terminal.Terminal, args ...string) error {
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
	newSession, err := createSession(controlClient, c.user.PtyReq, c.user.LastWindowChange)
	if err != nil {

		c.log.Error("Creating session failed: %s", err)
		return err
	}

	c.defaultHandle.Stop()

	c.log.Info("Connected to %s", controlClient.RemoteAddr().String())

	err = attachSession(term, newSession, c.user.ShellConnection, c.user.ShellRequests, c.user.EnabledRcfiles[args[0]])
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
	user *users.User,
	defaultHandle *internal.DefaultSSHHandler,
	controllableClients *sync.Map,
	log logger.Logger) *connect {
	return &connect{
		user:                user,
		defaultHandle:       defaultHandle,
		controllableClients: controllableClients,
		log:                 log,
	}
}

func createSession(sshConn ssh.Conn, ptyReq, lastWindowChange ssh.Request) (sc ssh.Channel, err error) {

	splice, newrequests, err := sshConn.OpenChannel("session", nil)
	if err != nil {
		return sc, fmt.Errorf("Unable to start remote session on host %s (%s) : %s", sshConn.RemoteAddr(), sshConn.ClientVersion(), err)
	}

	//Replay the pty and any the very last window size change in order to correctly size the PTY on the controlled client
	_, err = internal.SendRequest(ptyReq, splice)
	if err != nil {
		return sc, fmt.Errorf("Unable to send PTY request: %s", err)
	}

	_, err = internal.SendRequest(lastWindowChange, splice)
	if err != nil {
		return sc, fmt.Errorf("Unable to send last window change request: %s", err)
	}

	go ssh.DiscardRequests(newrequests)

	return splice, nil
}

// This was a massive pain in the ass to fix.
// Effectively, io.Copy(client, us) will 'eat' a character as its waiting on the human client to send a character
// Which then causes the io.Copy to try and write to 'client' only to find that the client is closed. Thus returning control back to the terminal interface
// I didnt like this, had to modify the terminal library to let us write user input, and then use a writer interface to copy the input data to both the io.Copy
// And the terminal, so that the io.Copy thread will end, and that we get the input on the terminal side.
// Damn you unstoppable blocking reads!

//Frankly I hate this fix. But I cant think of a better way of solving this
// Other than bringing this structure into the terminal and having the terminal expose a "Raw" mode hmm
type terminalWriter struct {
	sync.Mutex

	writer              io.Writer
	term                *terminal.Terminal
	enableWriteTermLine bool
}

func (dw *terminalWriter) Enable() {
	dw.Lock()
	defer dw.Unlock()

	dw.enableWriteTermLine = true
}

func (dw *terminalWriter) Write(p []byte) (n int, err error) {
	dw.Lock()
	defer dw.Unlock()

	n, err = dw.writer.Write(p)
	if err != nil {
		if dw.enableWriteTermLine {
			dw.term.SetLine(p)
		}
		return
	}

	if n != len(p) {
		return n, io.ErrShortWrite
	}

	return n, nil
}

func newTermMultiWriter(writer io.Writer, term *terminal.Terminal) *terminalWriter {

	return &terminalWriter{writer: writer, term: term}
}

func attachSession(term *terminal.Terminal, newSession, currentClientSession ssh.Channel, currentClientRequests <-chan *ssh.Request, rcfiles []string) error {

	finished := make(chan bool)
	sm := newTermMultiWriter(newSession, term)

	close := func() {
		newSession.Close()
		sm.Enable()      // This sucks... a lot. But cant think of a better way to do this
		finished <- true // Stop the request passer on IO error
	}

	//Setup the pipes for stdin/stdout over the connections

	//Start copying output before we start copying user input, so we can get the responses to the rc files lines
	var once sync.Once
	defer once.Do(close)

	go func() {
		//dst <- src
		io.Copy(sm, currentClientSession)
		once.Do(close)

	}()

	for _, path := range rcfiles {
		file, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(term, "Unable to open rc file: %s\n", path)
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
