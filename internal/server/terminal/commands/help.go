package commands

import (
	"fmt"
	"sort"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal/commands/constants"
	"github.com/NHAS/reverse_ssh/pkg/table"
)

type help struct {
}

func (h *help) Run(term *terminal.Terminal, args ...string) error {
	if len(args) < 1 {

		t, err := table.NewTable("Commands", "Function", "Purpose")
		if err != nil {
			return err
		}

		keys := []string{}
		for funcName := range term.GetHelpList() {
			keys = append(keys, funcName)
		}

		sort.Strings(keys)

		for _, k := range keys {
			hf, err := term.GetHelp(k)
			if err != nil {
				return err
			}

			err = t.AddValues(k, hf(true))
			if err != nil {
				return err
			}
		}

		t.Fprint(term)

		return nil
	}

	hf, err := term.GetHelp(args[0])
	if err != nil {
		return err
	}

	fmt.Fprintf(term, hf(false))

	return nil
}

func (h *help) Expect(sections []string) []string {
	if len(sections) == 1 {
		return []string{constants.Functions}
	}
	return nil
}

func (h *help) Help(brief bool) string {
	if brief {
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
