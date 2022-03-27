package handlers

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

//Session has a lot of 'function' in ssh. It can be used for shell, exec, subsystem, pty-req and more.
//However these calls are done through requests, rather than opening a new channel
//This callback just sorts out what the client wants to be doing
func Session(user *internal.User, newChannel ssh.NewChannel, log logger.Logger) {

	defer log.Info("Session disconnected")

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

			req.Reply(true, nil)

			parts := strings.Split(command.Cmd, " ")
			if len(parts) > 0 {
				if parts[0] == "scp" {

					scp(parts, connection, log)

					return
				}

				//Set a path if no path is set to search
				if len(os.Getenv("PATH")) == 0 {
					os.Setenv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/bin:/bin:/sbin")
				}

				cmd := exec.Command(parts[0], parts[1:]...)

				stdout, err := cmd.StdoutPipe()
				if err != nil {
					fmt.Fprintf(connection, "%s", err.Error())
					return
				}
				defer stdout.Close()

				cmd.Stderr = cmd.Stdout

				stdin, err := cmd.StdinPipe()
				if err != nil {
					fmt.Fprintf(connection, "%s", err.Error())
					return
				}
				defer stdin.Close()

				go io.Copy(stdin, connection)
				go io.Copy(connection, stdout)

				err = cmd.Run()
				if err != nil {
					fmt.Fprintf(connection, "%s", err.Error())
					return
				}
			}

			return
		case "shell":
			// We only accept the default shell
			// (i.e. no command in the Payload)
			req.Reply(len(req.Payload) == 0, nil)

			//This blocks so will keep the channel from defer closing
			shell(user, connection, requests, log)

			return
			//Yes, this is here for a reason future me. Despite the RFC saying "Only one of shell,subsystem, exec can occur per channel" pty-req actually proceeds all of them
		case "pty-req":

			//Ignoring the error here as we are not fully parsing the payload, leaving the unmarshal func a bit confused (thus returning an error)
			pty, err := internal.ParsePtyReq(req.Payload)
			if err != nil {
				log.Warning("Got undecodable pty request: %s", err)
				req.Reply(false, nil)
				return
			}
			user.Pty = &pty

			req.Reply(true, nil)
		default:
			log.Warning("Got an unknown request %s", req.Type)
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}

}
