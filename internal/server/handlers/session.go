package handlers

import (
	"encoding/binary"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/commands"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/server/webserver"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

// Session has a lot of 'function' in ssh. It can be used for shell, exec, subsystem, pty-req and more.
// However these calls are done through requests, rather than opening a new channel
// This callback just sorts out what the client wants to be doing

func sendExitCode(code uint32, channel ssh.Channel) {

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, code)

	channel.SendRequest("exit-status", false, b)
}

func Session(datadir string) ChannelHandler {
	return func(connectionDetails string, user *users.User, newChannel ssh.NewChannel, log logger.Logger) {

		sess, err := user.Session(connectionDetails)
		if err != nil {
			log.Warning("Could not get user session for %s: err: %s", connectionDetails, err)
			return
		}

		defer log.Info("Session disconnected: %s", sess.ConnectionDetails)

		// At this point, we have the opportunity to reject the client's
		// request for another logical connection
		connection, requests, err := newChannel.Accept()
		if err != nil {
			log.Warning("Could not accept channel (%s)", err)
			return
		}
		defer connection.Close()

		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered a panic:", r)
				debug.PrintStack()
			}
		}()

		sess.ShellRequests = requests

		for req := range requests {
			log.Info("Session got request: %q", req.Type)
			switch req.Type {
			case "exec":
				var command struct {
					Cmd string
				}
				err = ssh.Unmarshal(req.Payload, &command)
				if err != nil {
					log.Warning("Human client sent an undecodable exec payload: %s\n", err)
					req.Reply(false, nil)
					return
				}

				line := terminal.ParseLine(command.Cmd, 0)
				if line.Command != nil {
					c := commands.CreateCommands(sess.ConnectionDetails, user, log, datadir)

					if m, ok := c[line.Command.Value()]; ok {

						req.Reply(true, nil)
						err := m.Run(user, connection, line)
						if err != nil {
							sendExitCode(1, connection)
							fmt.Fprintf(connection, "%s", err.Error())
							return
						}
						sendExitCode(0, connection)

						return
					}
				}
				req.Reply(false, []byte("Unknown RSSH command"))
				sendExitCode(1, connection)
				return
			case "shell":
				// We only accept the default shell
				// (i.e. no command in the Payload)
				req.Reply(len(req.Payload) == 0, nil)

				term := terminal.NewAdvancedTerminal(connection, user, sess, internal.ConsoleLabel+"$ ")

				term.SetSize(int(sess.Pty.Columns), int(sess.Pty.Rows))

				term.AddValueAutoComplete(autocomplete.RemoteId, user.Autocomplete(), users.PublicClientsAutoComplete)
				term.AddValueAutoComplete(autocomplete.WebServerFileIds, webserver.Autocomplete)

				term.AddCommands(commands.CreateCommands(sess.ConnectionDetails, user, log, datadir))

				err := term.Run()
				if err != nil && err != io.EOF {
					sendExitCode(1, connection)
					log.Error("Error: %s", err)
				}
				sendExitCode(0, connection)

				return
				//Yes, this is here for a reason future me. Despite the RFC saying "Only one of shell,subsystem, exec can occur per channel" pty-req actuall proceeds all of them
			case "pty-req":

				//Ignoring the error here as we are not fully parsing the payload, leaving the unmarshal func a bit confused (thus returning an error)
				pty, err := internal.ParsePtyReq(req.Payload)
				if err != nil {
					log.Warning("Got undecodable pty request: %s", err)
					req.Reply(false, nil)
					return
				}
				sess.Pty = &pty

				req.Reply(true, nil)
			default:
				log.Warning("Unsupported request %s", req.Type)
				if req.WantReply {
					req.Reply(false, []byte("Unsupported request"))
				}
			}
		}
	}
}
