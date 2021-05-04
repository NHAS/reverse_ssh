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

func Exit() *exit {
	return &exit{}
}
