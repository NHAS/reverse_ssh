package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/client/connection"
	"github.com/NHAS/reverse_ssh/internal/client/handlers/subsystems"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"github.com/NHAS/reverse_ssh/pkg/storage"

	"golang.org/x/crypto/ssh"
)

func exit(session ssh.Channel, code int) {
	status := struct{ Status uint32 }{uint32(code)}
	session.SendRequest("exit-status", false, ssh.Marshal(&status))
}

// Session has a lot of 'function' in ssh. It can be used for shell, exec, subsystem, pty-req and more.
// However these calls are done through requests, rather than opening a new channel
// This callback just sorts out what the client wants to be doing
func Session(session *connection.Session) func(newChannel ssh.NewChannel, log logger.Logger) {

	return func(newChannel ssh.NewChannel, log logger.Logger) {

		defer log.Info("Session disconnected")

		// At this point, we have the opportunity to reject the client's
		// request for another logical connection
		connection, requests, err := newChannel.Accept()
		if err != nil {
			log.Warning("Could not accept channel (%s)", err)
			return
		}
		defer func() {
			exit(connection, 0)
			connection.Close()
		}()

		for req := range requests {
			log.Info("Session got request: %q", req.Type)
			switch req.Type {

			case "subsystem":

				err := subsystems.RunSubsystems(connection, req)
				if err != nil {
					log.Error("subsystem encountered an error: %s", err.Error())
					fmt.Fprintf(connection, "subsystem error: '%s'", err.Error())
				}

				return

			case "exec":
				var cmd internal.ShellStruct
				err = ssh.Unmarshal(req.Payload, &cmd)
				if err != nil {
					log.Warning("Human client sent an undecodable exec payload: %s\n", err)
					req.Reply(false, nil)
					return
				}

				req.Reply(true, nil)

				line := terminal.ParseLine(cmd.Cmd, 0)

				if line.Empty() {
					log.Warning("Human client sent an empty exec payload: %s\n", err)
					return
				}

				command := line.Command.Value()

				if command == "scp" {
					scp(line.Chunks[1:], connection, log)
					return
				}

				u, ok := isUrl(command)
				if ok {
					command, err = download(session.ServerConnection, u)
					if err != nil {
						fmt.Fprintf(connection, "%s", err.Error())
						return
					}
				}

				argv := ""
				if u != nil {
					argv = u.Query().Get("argv")
				}

				if session.Pty != nil {
					runCommandWithPty(argv, command, line.Chunks[1:], session.Pty, requests, log, connection)
					return
				}
				runCommand(argv, command, line.Chunks[1:], connection)

				return
			case "shell":

				//We accept the shell request
				req.Reply(true, nil)

				var shellPath internal.ShellStruct
				err := ssh.Unmarshal(req.Payload, &shellPath)
				if err != nil || shellPath.Cmd == "" {

					//This blocks so will keep the channel from defer closing
					shell(session.Pty, connection, requests, log)
					return
				}
				parts := strings.Split(shellPath.Cmd, " ")
				if len(parts) > 0 {
					command := parts[0]
					u, ok := isUrl(parts[0])
					if ok {
						command, err = download(session.ServerConnection, u)
						if err != nil {
							fmt.Fprintf(connection, "%s", err.Error())
							return
						}
					}

					argv := ""
					if u != nil {
						argv = u.Query().Get("argv")
					}

					runCommandWithPty(argv, command, parts[1:], session.Pty, requests, log, connection)
				}
				return
				//Yes, this is here for a reason future me. Despite the RFC saying "Only one of shell,subsystem, exec can occur per channel" pty-req actually proceeds all of them
			case "pty-req":

				pty, err := internal.ParsePtyReq(req.Payload)
				if err != nil {
					log.Warning("Got undecodable pty request: %s", err)
					req.Reply(false, nil)
					return
				}
				session.Pty = &pty

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

func runCommand(argv string, command string, args []string, connection ssh.Channel) {
	//Set a path if no path is set to search
	if len(os.Getenv("PATH")) == 0 {
		if runtime.GOOS != "windows" {
			os.Setenv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/bin:/bin:/sbin")
		} else {
			os.Setenv("PATH", "C:\\Windows\\system32;C:\\Windows;C:\\Windows\\System32\\Wbem;C:\\Windows\\System32\\WindowsPowerShell\v1.0\\")
		}
	}

	cmd := exec.Command(command, args...)
	if len(argv) != 0 {
		cmd.Args[0] = argv
	}

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

func isUrl(data string) (*url.URL, bool) {
	u, err := url.Parse(data)
	if err != nil {
		return nil, false
	}

	switch u.Scheme {
	case "http", "https", "rssh":
		return u, true
	}
	return u, false
}

func download(serverConnection ssh.Conn, fromUrl *url.URL) (result string, err error) {
	if fromUrl == nil {
		return "", errors.New("url was nil")
	}

	var (
		reader   io.ReadCloser
		filename string
	)

	urlCopy := *fromUrl

	query := urlCopy.Query()
	query.Del("argv")

	urlCopy.RawQuery = query.Encode()

	switch urlCopy.Scheme {
	case "http", "https":

		resp, err := http.Get(urlCopy.String())
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		reader = resp.Body

		filename = path.Base(urlCopy.Path)
		if filename == "." {
			filename, err = internal.RandomString(16)
			if err != nil {
				return "", err
			}
		}

	case "rssh":
		filename = path.Base(strings.TrimSuffix(urlCopy.String(), "rssh://"))

		ch, reqs, err := serverConnection.OpenChannel("rssh-download", []byte(filename))
		if err != nil {
			return "", err
		}
		go ssh.DiscardRequests(reqs)

		reader = ch

	default:
		return "", errors.New("unknown uri handler: " + fromUrl.Scheme)

	}

	return storage.Store(filename, reader)
}
