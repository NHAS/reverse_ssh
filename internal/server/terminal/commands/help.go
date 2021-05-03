package commands

import (
	"fmt"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
)

func Help(term *terminal.Terminal, args ...string) error {
	fmt.Fprintln(term, "Commands: ")
	for _, completion := range term.GetFunctions() {
		fmt.Fprintf(term, "%s\n", completion)
	}

	return nil
}
