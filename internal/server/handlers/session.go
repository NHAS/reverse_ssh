package handlers

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal/commands"
	"github.com/NHAS/reverse_ssh/internal/server/terminal/commands/constants"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

//Session has a lot of 'function' in ssh. It can be used for shell, exec, subsystem, pty-req and more.
//However these calls are done through requests, rather than opening a new channel
//This callback just sorts out what the client wants to be doing
func Session(controllableClients *sync.Map, autoCompleteClients *trie.Trie) internal.ChannelHandler {

	return func(user *users.User, newChannel ssh.NewChannel, log logger.Logger) {

		defer log.Info("Human disconnected, client version %s", user.ServerConnection.ClientVersion())

		// At this point, we have the opportunity to reject the client's
		// request for another logical connection
		connection, requests, err := newChannel.Accept()
		if err != nil {
			log.Warning("Could not accept channel (%s)", err)
			return
		}
		defer connection.Close()

		var ptySettings internal.PtyReq

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
				if len(parts) > 1 {
					if parts[0] != "scp" {
						log.Warning("Human client tried to execute something other than SCP: %s\n", parts[0])
						return
					}

					//Find what the target file path is, essentially ignore anything that is a flag '-t'
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
				shell(user, connection, requests, ptySettings, controllableClients, autoCompleteClients, log)

				return
			case "pty-req":

				//Ignoring the error here as we are not fully parsing the payload, leaving the unmarshal func a bit confused (thus returning an error)
				ptySettings, err = internal.ParsePtyReq(req.Payload)
				if err != nil {
					log.Warning("Got undecodable pty request: %s", err)
					req.Reply(false, nil)
					return
				}

				req.Reply(true, nil)
			default:
				log.Warning("Got an unknown request %s", req.Type)
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
		}

	}

}

func scp(connection ssh.Channel, requests <-chan *ssh.Request, mode string, path string, controllableClients *sync.Map) error {
	go ssh.DiscardRequests(requests)

	parts := strings.SplitN(path, ":", 2)

	if len(parts) < 1 {
		internal.ScpError("No target specified", connection)
		return nil
	}

	conn, ok := controllableClients.Load(parts[0])
	if !ok {
		internal.ScpError(fmt.Sprintf("Invalid target, %s not found", parts[0]), connection)
		return nil
	}

	device := conn.(ssh.Conn)

	//This is not the standard spec, but I wanted to do it this way as its easier to deal with
	scp, r, err := device.OpenChannel("scp", ssh.Marshal(&internal.Scp{Mode: mode, Path: parts[1]}))
	if err != nil {
		internal.ScpError("Could not connect to remote target", connection)
		return err
	}
	go ssh.DiscardRequests(r)

	go func() {
		defer scp.Close()
		defer connection.Close()

		io.Copy(connection, scp)
	}()

	defer scp.Close()
	defer connection.Close()
	io.Copy(scp, connection)

	return nil
}

func shell(user *users.User, connection ssh.Channel, requests <-chan *ssh.Request, ptySettings internal.PtyReq, controllableClients *sync.Map, autoCompleteClients *trie.Trie, log logger.Logger) error {

	user.PtyReq = ssh.Request{Type: "pty-req", WantReply: true, Payload: ssh.Marshal(ptySettings)}
	user.ShellConnection = connection
	user.ShellRequests = requests

	term := terminal.NewAdvancedTerminal(connection, "catcher$ ")

	term.SetSize(int(ptySettings.Columns), int(ptySettings.Rows))

	term.AddValueAutoComplete(constants.RemoteId, autoCompleteClients)

	defaultHandle := internal.NewDefaultHandler(user, term)

	term.AddCommand("ls", commands.List(controllableClients))
	term.AddCommand("help", commands.Help())
	term.AddCommand("exit", commands.Exit())
	term.AddCommand("connect", commands.Connect(user, defaultHandle, controllableClients, log))
	term.AddCommand("rc", commands.RC(user, controllableClients))
	term.AddCommand("proxy", commands.Proxy(user, controllableClients))

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	// While we arent passing the requests directly to the remote host consume them with our terminal and store the results to send initialy to the remote on client connect
	defaultHandle.Start()

	//Send list of controllable remote hosts to human client
	commands.List(controllableClients).Run(term)

	//Blocking function to handle all the human function calls. Will return io.EOF on exit, otherwise an error is passed up we cant deal with
	err := term.Run()
	if err != nil && err != io.EOF {
		fmt.Fprintf(term, "Error: %s\n", err)
	}

	return err

}
