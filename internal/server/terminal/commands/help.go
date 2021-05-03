package commands

import (
	"fmt"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
)

type help struct {
}

func (h *help) Run(term *terminal.Terminal, args ...string) error {
	fmt.Fprintln(term, "Commands: ")
	for _, completion := range term.GetFunctions() {
		fmt.Fprintf(term, "%s\n", completion)
	}

	return nil
}

func (h *help) Expect(section int) string {
	return ""
}

func Help() *help {
	return &help{}
}
