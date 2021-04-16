// +build windows
package client

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"syscall"

	"golang.org/x/crypto/ssh"
)

//The basic windows shell handler, as there arent any good golang libraries to work with windows conpty
func shellChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {

	// At this point, we have the opportunity to reject the client's.
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}
	defer connection.Close()

	go func() {
		defer connection.Close()
		r := bufio.NewReader(connection)
		for {
			fmt.Fprintf(connection, "\n> ")
			order, err := r.ReadString('\n')
			if nil != err {
				return
			}

			cmd := exec.Command("cmd", "/C", order)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			out, err := cmd.CombinedOutput()
			if err != nil {
				out = []byte(fmt.Sprintf("Unable to execute command. Reason: %s", err))
			}

			fmt.Fprintf(connection, "%s", out)
			if err != nil {
				log.Println("Unable to write: ", err)
				return
			}
		}
	}()

	for req := range requests {
		log.Println("Got request: ", req.Type)
		switch req.Type {
		case "shell":
			// We only accept the default shell
			// (i.e. no command in the Payload)
			req.Reply(len(req.Payload) == 0, nil)

		case "window-change":
			req.Reply(true, nil)

		case "pty-req":
			req.Reply(true, nil)
		}
	}

}
