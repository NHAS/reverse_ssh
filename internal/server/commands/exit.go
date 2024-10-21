package commands

import (
	"io"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
)

type exit struct {
}

func (e *exit) ValidArgs() map[string]string {
	return map[string]string{}
}

func (e *exit) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {
	return io.EOF
}

func (e *exit) Expect(line terminal.ParsedLine) []string {
	return nil
}

func (e *exit) Help(explain bool) string {

	const description = "Close server console connection"

	if explain {
		return "Close server console"
	}

	return terminal.MakeHelpText(e.ValidArgs(),
		"exit",
		description,
	)
}
