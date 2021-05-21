package handlers

import (
	"log"
	"strings"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

//Session has a lot of 'function' in ssh. It can be used for shell, exec, subsystem, pty-req and more.
//However these calls are done through requests, rather than opening a new channel
//This callback just sorts out what the client wants to be doing
func Session(controllableClients *sync.Map, autoCompleteClients *trie.Trie) internal.ChannelHandler {

	return func(user *users.User, newChannel ssh.NewChannel) {

		defer log.Printf("Human client disconnected %s (%s)\n", user.ServerConnection.RemoteAddr(), user.ServerConnection.ClientVersion())

		// At this point, we have the opportunity to reject the client's
		// request for another logical connection
		connection, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Could not accept channel (%s)", err)
			return
		}
		defer connection.Close()

		var ptySettings internal.PtyReq

		for req := range requests {
			log.Println("Session got request: ", req.Type)
			switch req.Type {
			case "exec":
				var command struct {
					Cmd string
				}
				err = ssh.Unmarshal(req.Payload, &command)
				if err != nil {
					log.Printf("Human client sent an undecodable exec payload: %s", err)
					req.Reply(false, nil)
					return
				}

				parts := strings.Split(command.Cmd, " ")
				if len(parts) > 1 {
					if parts[0] != "scp" {
						log.Printf("Human client tried to execute something other than SCP: %s", parts[0])
						return
					}
					log.Printf("%s", command.Cmd)

					//Find where the path is, essentially ignore anything that is a flag '-t'
					loc := -1
					mode := ""
					for i := 1; i < len(parts); i++ {
						if mode == "" && (parts[i] == "-t" || parts[i] == "-f") {
							mode = parts[i]
							continue
						}

						if len(parts[i]) > 0 && parts[i][0] != '-' {
							loc = i
							break
						}
					}

					if loc != -1 {
						req.Reply(true, nil)
						scp(connection, requests, mode, strings.Join(parts[loc:], " "), controllableClients)
					}
					return
				}
				req.Reply(false, nil)
				return
			case "shell":
				// We only accept the default shell
				// (i.e. no command in the Payload)
				req.Reply(len(req.Payload) == 0, nil)

				//This blocks so will keep the channel from defer closing
				shell(user, connection, requests, ptySettings, controllableClients, autoCompleteClients)

				return
			case "pty-req":

				//Ignoring the error here as we are not fully parsing the payload, leaving the unmarshal func a bit confused (thus returning an error)
				ptySettings, err = internal.ParsePtyReq(req.Payload)
				if err != nil {
					log.Printf("Human client %s, sent undecodable pty request: %s\n", user.ServerConnection.RemoteAddr(), err)
					return
				}

				req.Reply(true, nil)
			default:
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
		}

	}

}
