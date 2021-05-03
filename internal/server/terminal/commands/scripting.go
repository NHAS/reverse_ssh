package commands

import (
	"fmt"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
)

func RCFile(term *terminal.Terminal) (terminal.TerminalFunctionCallback, map[string][]string) {

	rcfiles := make(map[string][]string)      // Map of rc label -> rc file lines
	enabledHosts := make(map[string][]string) // Map of host id string -> rc file lines
	return func(args ...string) error {
		if len(args) != 1 {
			helpText := "rc load <label> <rc file path>\n"
			helpText += "rc enable <label> <host>\n"
			helpText += "rc disable <label> <host>"
			return fmt.Errorf(helpText)
		}

		switch args[0] {
		case "load":
			if len(args) != 3 {
				return fmt.Errorf("Not enough args for rc load. rc load <label> <rc file path>")
			}

			if _, ok := rcfiles[args[1]]; ok {
				return fmt.Errorf("Label already in use: %s", args[1])
			}

		case "enable":
		case "disable":
		default:
			return fmt.Errorf("Unknown rc sub command: %s", args[0])
		}

		return nil
	}, enabledHosts
}
