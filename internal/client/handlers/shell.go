//go:build !windows
// +build !windows

package handlers

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"github.com/creack/pty"
	"golang.org/x/crypto/ssh"
)

var shells []string

func init() {

	file, err := os.Open("/etc/shells")
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()

			if len(line) > 0 && line[0] == '#' || strings.TrimSpace(line) == "" {
				continue
			}
			shells = append(shells, strings.TrimSpace(line))
		}
	} else {
		shells = []string{
			"/bin/bash",
			"/bin/sh",
			"/bin/zsh",
			"/bin/ash",
		}

	}

	log.Println("Detected Shells: ")
	for _, s := range shells {

		if stats, err := os.Stat(s); err != nil && (os.IsNotExist(err) || !stats.IsDir()) {

			fmt.Printf("Rejecting Shell: '%s' Reason: %v\n", s, err)
			continue

		}
		shells = append(shells, s)
		fmt.Println("\t\t", s)
	}

}

//This basically handles exactly like a SSH server would
func shell(user *internal.User, connection ssh.Channel, requests <-chan *ssh.Request, log logger.Logger) {

	if user.Pty == nil {
		fmt.Fprintf(connection, "Shell without pty not allowed.")
		return
	}

	path := ""
	if len(shells) != 0 {
		path = shells[0]
	}

	// Fire up a shell for this session
	shell := exec.Command(path)
	shell.Env = os.Environ()
	shell.Env = append(shell.Env, "TERM="+user.Pty.Term)

	close := func() {
		connection.Close()
		if shell.Process != nil {
			err := shell.Process.Kill()
			if err != nil {
				log.Warning("Failed to kill shell(%s)", err)
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

	err = pty.Setsize(shellf, &pty.Winsize{Cols: uint16(user.Pty.Columns), Rows: uint16(user.Pty.Rows)})
	if err != nil {
		log.Error("Unable to set terminal size %s", err)
		fmt.Fprintf(connection, "Unable to set term size")
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

	for req := range requests {
		switch req.Type {

		case "window-change":
			w, h := internal.ParseDims(req.Payload)
			err = pty.Setsize(shellf, &pty.Winsize{Cols: uint16(w), Rows: uint16(h)})
			if err != nil {
				log.Warning("Unable to set terminal size: %s", err)
			}

		default:
			log.Warning("Unknown request %s", req.Type)
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}

}
