package handlers

import (
	"fmt"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/commands"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

//Session has a lot of 'function' in ssh. It can be used for shell, exec, subsystem, pty-req and more.
//However these calls are done through requests, rather than opening a new channel
//This callback just sorts out what the client wants to be doing
func Session(user *internal.User, newChannel ssh.NewChannel, log logger.Logger) {

	defer log.Info("Session disconnected: %s", user.ServerConnection.ClientVersion())

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Warning("Could not accept channel (%s)", err)
		return
	}
	defer connection.Close()

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

			parts := strings.Split(command.Cmd, " ")
			if len(parts) > 0 {
				c := commands.CreateCommands(user, connection, requests, log)

				if m, ok := c[parts[0]]; ok {

					req.Reply(true, nil)
					err := m.Run(connection, parts[1:]...)
					if err != nil {
						fmt.Fprintf(connection, "%s", err.Error())
						return
					}
					return
				}
			}
			req.Reply(false, []byte("Unknown RSSH command"))
			return
		default:
			log.Warning("Unsupported request %s", req.Type)
			if req.WantReply {
				req.Reply(false, []byte("Unsupported request"))
			}
		}
	}

}
