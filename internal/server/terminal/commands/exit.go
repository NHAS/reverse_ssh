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

func (e *exit) Expect(section int) string {
	return ""
}

func Exit() *exit {
	return &exit{}
}
