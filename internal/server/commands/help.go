package commands

import (
	"fmt"
	"io"
	"sort"

	"github.com/NHAS/reverse_ssh/internal/server/commands/constants"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/pkg/table"
)

type help struct {
	term *terminal.Terminal
}

func (h *help) Run(tty io.ReadWriter, args ...string) error {
	if len(args) < 1 {

		t, err := table.NewTable("Commands", "Function", "Purpose")
		if err != nil {
			return err
		}

		keys := []string{}
		for funcName := range h.term.GetHelpList() {
			keys = append(keys, funcName)
		}

		sort.Strings(keys)

		for _, k := range keys {
			hf, err := h.term.GetHelp(k)
			if err != nil {
				return err
			}

			err = t.AddValues(k, hf(true))
			if err != nil {
				return err
			}
		}

		t.Fprint(tty)

		return nil
	}

	hf, err := h.term.GetHelp(args[0])
	if err != nil {
		return err
	}

	fmt.Fprintf(tty, "\ndescription:\n%s\n", hf(true))

	fmt.Fprintf(tty, "\nusage:\n%s\n", hf(false))

	return nil
}

func (h *help) Expect(sections []string) []string {
	if len(sections) == 1 {
		return []string{constants.Functions}
	}
	return nil
}

func (h *help) Help(explain bool) string {
	if explain {
		return "Get help for commands, or display all commands"
	}

	return makeHelpText(
		"help",
		"help <functions>",
	)
}

func Help() *help {
	return &help{}
}
