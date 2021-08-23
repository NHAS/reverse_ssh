// +build !windows

package client

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"github.com/creack/pty"
	"golang.org/x/crypto/ssh"
)

//This basically handles exactly like a SSH server would
func shellChannel(user *internal.User, newChannel ssh.NewChannel, log logger.Logger) {

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Warning("Could not accept channel (%s)", err)
		return
	}

	var ptyreq internal.PtyReq
PtyListener:
	for req := range requests {

		switch req.Type {
		case "pty-req":
			ptyreq, _ = internal.ParsePtyReq(req.Payload)

			req.Reply(true, nil)
			break PtyListener
		default:
			log.Warning("Unknown message: '%s'", req.Type)
		}
	}

	path := ""
	if len(shells) == 0 {
		term := terminal.NewTerminal(connection, "> ")
		fmt.Fprintln(term, "Unable to determine shell to execute")
		for {
			line, err := term.ReadLine()
			if err != nil {
				log.Warning("Unable to handle input")
				return
			}

			if stats, err := os.Stat(line); !os.IsExist(err) || stats.IsDir() {
				fmt.Fprintln(term, "Unsuitable selection: ", err)
				continue
			}
			path = line
			break

		}
	} else {
		path = shells[0]
	}

	// Fire up a shell for this session
	shell := exec.Command(path)
	shell.Env = os.Environ()
	shell.Env = append(shell.Env, "TERM="+ptyreq.Term)

	// Prepare teardown function

	close := func() {
		connection.Close() // Not a fan of this
		if shell.Process != nil {
			_, err := shell.Process.Wait()
			if err != nil {
				log.Warning("Failed to exit bash (%s)", err)
			}
		}

		log.Info("Session closed")
	}

	// Allocate a terminal for this channel
	log.Info("Creating pty...")
	shellf, err := pty.Start(shell)
	if err != nil {
		log.Info("Could not start pty (%s)", err)
		close()
		return
	}

	//pipe session to bash and visa-versa
	var once sync.Once
	go func() {
		io.Copy(connection, shellf)
		once.Do(close)
	}()
	go func() {
		io.Copy(shellf, connection)
		once.Do(close)
	}()
	defer once.Do(close)

	err = pty.Setsize(shellf, &pty.Winsize{Cols: uint16(ptyreq.Columns), Rows: uint16(ptyreq.Rows)})
	if err != nil {
		log.Error("Unable to set terminal size %s", err)
		fmt.Fprintf(connection, "Unable to set term size")
	}

	for req := range requests {
		log.Info("Got request: %s", req.Type)
		switch req.Type {
		case "shell":
			// We only accept the default shell
			// (i.e. no command in the Payload)
			req.Reply(true, []byte(path))

		case "window-change":
			w, h := internal.ParseDims(req.Payload)
			err = pty.Setsize(shellf, &pty.Winsize{Cols: uint16(w), Rows: uint16(h)})
			if err != nil {
				log.Warning("Unable to set terminal size: %s", err)
			}
		}
	}

}
