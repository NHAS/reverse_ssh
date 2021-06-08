package commands

import (
	"io"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
)

type exit struct {
}

func (e *exit) Run(term *terminal.Terminal, args ...string) error {
	return io.EOF
}

func (e *exit) Expect(sections []string) []string {
	return nil
}

func (e *exit) Help(explain bool) string {
	if explain {
		return "Quit connection"
	}

	return makeHelpText("exit")
}

func Exit() *exit {
	return &exit{}
}
