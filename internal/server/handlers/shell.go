package handlers

import (
	"fmt"
	"io"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal/commands"
	"github.com/NHAS/reverse_ssh/internal/server/terminal/commands/constants"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

func shell(user *internal.User, connection ssh.Channel, requests <-chan *ssh.Request, controllableClients *sync.Map, autoCompleteClients *trie.Trie, log logger.Logger) error {

	user.ShellConnection = connection
	user.ShellRequests = requests

	term := terminal.NewAdvancedTerminal(connection, "catcher$ ")

	term.SetSize(int(user.Pty.Columns), int(user.Pty.Rows))

	term.AddValueAutoComplete(constants.RemoteId, autoCompleteClients)

	defaultHandle := commands.NewWindowSizeChangeHandler(user, term)

	term.AddCommand("ls", commands.List(controllableClients))
	term.AddCommand("help", commands.Help())
	term.AddCommand("exit", commands.Exit())
	term.AddCommand("connect", commands.Connect(user, controllableClients, defaultHandle, log))
	term.AddCommand("kill", commands.Kill(controllableClients, log))
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