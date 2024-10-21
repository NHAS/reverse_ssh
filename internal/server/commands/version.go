package commands

import (
	"fmt"
	"io"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
)

type version struct {
}

func (v *version) ValidArgs() map[string]string {
	return map[string]string{}
}

func (v *version) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {
	fmt.Fprintln(tty, internal.Version)
	return nil
}

func (v *version) Expect(line terminal.ParsedLine) []string {
	return nil
}

func (v *version) Help(explain bool) string {
	const description = "Give server build version"

	if explain {
		return description
	}

	return terminal.MakeHelpText(v.ValidArgs(),
		"version",
		description)
}
