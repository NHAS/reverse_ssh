package server

import (
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"

	"golang.org/x/crypto/ssh"
)

func createSession(sshConn ssh.Conn, ptyReq, lastWindowChange ssh.Request) (sc ssh.Channel, err error) {

	splice, newrequests, err := sshConn.OpenChannel("session", nil)
	if err != nil {
		log.Printf("Unable to start remote session on host %s (%s) : %s\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), err)
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
type TerminalWriter struct {
	sync.Mutex

	writer              io.Writer
	term                *terminal.Terminal
	enableWriteTermLine bool
}

func (dw *TerminalWriter) Enable() {
	dw.Lock()
	defer dw.Unlock()

	dw.enableWriteTermLine = true
}

func (dw *TerminalWriter) Write(p []byte) (n int, err error) {
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

func NewTermMultiWriter(writer io.Writer, term *terminal.Terminal) *TerminalWriter {

	return &TerminalWriter{writer: writer, term: term}
}

func attachSession(term *terminal.Terminal, newSession, currentClientSession ssh.Channel, currentClientRequests <-chan *ssh.Request) error {

	finished := make(chan bool)
	sm := NewTermMultiWriter(newSession, term)

	close := func() {
		newSession.Close()
		sm.Enable()      // This sucks... a lot. But cant think of a better way to do this
		finished <- true // Stop the request passer on IO error
	}

	//Setup the pipes for stdin/stdout over the connections
	//newSession being the remote host being controlled
	var once sync.Once
	go func() {
		io.Copy(currentClientSession, newSession) // Potentially be more verbose about errors here
		once.Do(close)                            // Only close the newSession connection once

	}()
	go func() {

		io.Copy(sm, currentClientSession)
		once.Do(close)

	}()
	defer once.Do(close)

RequestsPasser:
	for {
		select {
		case r := <-currentClientRequests:
			response, err := internal.SendRequest(*r, newSession)
			if err != nil {
				break RequestsPasser
			}

			if r.WantReply {
				r.Reply(response, nil)
			}
		case <-finished:
			break RequestsPasser
		}

	}

	return nil
}
