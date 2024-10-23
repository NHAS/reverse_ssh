package commands

import (
	"io"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
)

type clear struct {
}

func (e *clear) ValidArgs() map[string]string {
	return map[string]string{}
}

func (e *clear) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {

	term, ok := tty.(*terminal.Terminal)
	if !ok {
		return nil
	}

	term.Clear()

	return nil
}

func (e *clear) Expect(line terminal.ParsedLine) []string {
	return nil
}

func (e *clear) Help(explain bool) string {

	const description = "Clear server console"

	if explain {
		return description
	}

	return terminal.MakeHelpText(e.ValidArgs(),
		"clear",
		description,
	)
}
