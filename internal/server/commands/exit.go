package commands

import (
	"io"
)

type exit struct {
}

func (e *exit) Run(tty io.ReadWriter, args ...string) error {
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
