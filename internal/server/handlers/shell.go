package handlers

import (
	"fmt"
	"io"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/clients"
	"github.com/NHAS/reverse_ssh/internal/server/commands"
	"github.com/NHAS/reverse_ssh/internal/server/webserver"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func shell(user *internal.User, connection ssh.Channel, log logger.Logger) error {
	term := terminal.NewAdvancedTerminal(connection, user, "catcher$ ")

	term.AddValueAutoComplete(autocomplete.RemoteId, clients.Autocomplete)
	term.AddValueAutoComplete(autocomplete.WebServerFileIds, webserver.Autocomplete)

	term.AddCommands(commands.CreateCommands(user, log))

	err := term.Run()
	if err != nil && err != io.EOF {
		fmt.Fprintf(term, "Error: %s\n", err)
	}

	return err
}
