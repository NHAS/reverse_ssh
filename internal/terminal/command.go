package terminal

import "io"

type Command interface {
	// Returns the expected syntax for the command, used in the autocomplete process with text tokens to indicate where autocomplete can occur
	Expect(sections []string) []string
	// Run the command with the given arguments
	Run(output io.ReadWriter, args ...string) error
	// Give helptext for commands
	Help(explain bool) string
}
