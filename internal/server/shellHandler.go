package server

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"golang.org/x/crypto/ssh"
)

var defaultHandle = newDefaultHandler()

func List(term *terminal.Terminal) terminal.TerminalFunctionCallback {
	return func(args ...string) error {
		controllableClients.Range(func(idStr interface{}, value interface{}) bool {
			fmt.Fprintf(term, "%s, client version: %s\n",
				idStr,
				value.(ssh.Conn).ClientVersion(),
			)
			return true
		})

		return nil
	}
}

func Help(term *terminal.Terminal) terminal.TerminalFunctionCallback {
	return func(args ...string) error {
		fmt.Fprintln(term, "Commands: ")
		for _, completion := range term.GetFunctions() {
			fmt.Fprintf(term, "%s\n", completion)
		}

		return nil
	}
}

func Exit(args ...string) error {
	return io.EOF
}

func Connect(term *terminal.Terminal, ptyReq, lastWindowChange *ssh.Request, sshConn ssh.Conn, connection ssh.Channel, requests <-chan *ssh.Request) terminal.TerminalFunctionCallback {

	return func(args ...string) error {
		if len(args) != 1 {
			return fmt.Errorf("connect <remote machine id>")
		}

		c, ok := controllableClients.Load(args[0])
		if !ok {
			return fmt.Errorf("Unknown connection host")
		}

		controlClient := c.(ssh.Conn)

		defer func() {
			connections[sshConn] = nil

			log.Printf("Client %s (%s) has disconnected from remote host %s (%s)\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), controlClient.RemoteAddr(), controlClient.ClientVersion())

			defaultHandle.Start(ptyReq, lastWindowChange, term, requests) // Re-enable the default handler if the client isnt connected to a remote host

		}()

		//Attempt to connect to remote host and send inital pty request and screen size
		// If we cant, report and error to the clients terminal
		newSession, err := createSession(controlClient, *ptyReq, *lastWindowChange)
		if err != nil {
			return err
		}

		defaultHandle.Stop()

		connections[sshConn] = controlClient

		err = attachSession(term, newSession, connection, requests)
		if err != nil {

			log.Println("Client tried to attach session and failed: ", err)
			return err
		}

		return fmt.Errorf("Session has terminated.") // Not really an error. But we can get the terminal to print out stuff
	}
}

func sessionChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {

	defer log.Printf("Human client disconnected %s (%s)\n", sshConn.RemoteAddr(), sshConn.ClientVersion())

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}
	defer connection.Close()

	var ptyReq, lastWindowChange ssh.Request

	term := terminal.NewAdvancedTerminal(connection, autoCompleteClients, "> ")

	term.AddFunction("ls", List(term))
	term.AddFunction("help", Help(term))
	term.AddFunction("exit", Exit)
	term.AddFunction("connect", Connect(term, &ptyReq, &lastWindowChange, sshConn, connection, requests))

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	// While we arent passing the requests directly to the remote host consume them with our terminal and store the results to send initialy to the remote on client connect
	defaultHandle.Start(&ptyReq, &lastWindowChange, term, requests)

	//Send list of controllable remote hosts to human client
	fmt.Fprintf(term, "Connected controllable clients: \n")
	controllableClients.Range(func(idStr interface{}, value interface{}) bool {
		fmt.Fprintf(term, "%s, client version: %s\n",
			idStr,
			value.(ssh.Conn).ClientVersion(),
		)
		return true
	})

	//Blocking function to handle all the human function calls. Will return io.EOF on exit, otherwise an error is passed up we cant deal with
	err = term.Run()
	if err != nil && err != io.EOF {
		fmt.Fprintf(term, "Error: %s\n", err)
	}

	delete(connections, sshConn)
}

type defaultSSHHandler struct {
	cancel context.CancelFunc
	ctx    context.Context
}

func newDefaultHandler() defaultSSHHandler {
	c, cancelFunc := context.WithCancel(context.Background())

	return defaultSSHHandler{
		cancel: cancelFunc,
		ctx:    c,
	}
}

func (dh *defaultSSHHandler) Stop() {
	dh.cancel()
}

func (dh *defaultSSHHandler) Start(ptyr *ssh.Request, wc *ssh.Request, term *terminal.Terminal, requests <-chan *ssh.Request) {

	go func() {
		for {
			select {
			case <-dh.ctx.Done():
				return
			case req := <-requests:
				if req == nil { // Channel has closed, so therefore end this default handler
					return
				}

				log.Println("Got request: ", req.Type)
				switch req.Type {
				case "shell":
					// We only accept the default shell
					// (i.e. no command in the Payload)
					req.Reply(len(req.Payload) == 0, nil)
				case "pty-req":

					//Ignoring the error here as we are not fully parsing the payload, leaving the unmarshal func a bit confused (thus returning an error)
					ptyReqData, _ := internal.ParsePtyReq(req.Payload)
					term.SetSize(int(ptyReqData.Columns), int(ptyReqData.Rows))

					*ptyr = *req
					req.Reply(true, nil)
				case "window-change":
					w, h := internal.ParseDims(req.Payload)
					term.SetSize(int(w), int(h))

					*wc = *req
				}
			}

		}
	}()
}
