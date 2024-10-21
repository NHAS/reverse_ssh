package commands

import (
	"fmt"
	"io"
	"sort"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
	"github.com/NHAS/reverse_ssh/pkg/table"
)

type help struct {
}

func (h *help) ValidArgs() map[string]string {
	return map[string]string{"l": "List all function names only"}
}

func (h *help) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {

	if line.IsSet("l") {
		funcs := []string{}
		for funcName := range allCommands {
			funcs = append(funcs, funcName)
		}

		sort.Strings(funcs)

		for _, funcName := range funcs {
			fmt.Fprintln(tty, funcName)
		}

		return nil
	}

	if len(line.Arguments) < 1 {

		t, err := table.NewTable("Commands", "Function", "Purpose")
		if err != nil {
			return err
		}

		keys := []string{}
		for funcName := range allCommands {
			keys = append(keys, funcName)
		}

		sort.Strings(keys)

		for _, k := range keys {
			hf := allCommands[k].Help

			err = t.AddValues(k, hf(true))
			if err != nil {
				return err
			}
		}

		t.Fprint(tty)

		return nil
	}

	l, ok := allCommands[line.Arguments[0].Value()]
	if !ok {
		return fmt.Errorf("Command %s not found", line.Arguments[0].Value())
	}

	fmt.Fprintf(tty, "\ndescription:\n%s\n", l.Help(true))

	fmt.Fprintf(tty, "\nusage:\n%s\n", l.Help(false))

	return nil
}

func (h *help) Expect(line terminal.ParsedLine) []string {
	if len(line.Arguments) <= 1 {
		return []string{autocomplete.Functions}
	}
	return nil
}

func (h *help) Help(explain bool) string {

	const description = "Get help for commands, or display all commands"
	if explain {
		return description
	}

	return terminal.MakeHelpText(h.ValidArgs(),
		"help",
		"help <functions>",
		description,
	)
}
