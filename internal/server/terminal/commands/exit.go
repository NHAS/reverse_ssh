package commands

import (
	"io"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
)

func Exit(term *terminal.Terminal, args ...string) error {
	return io.EOF
}
