package terminal

import "io"

type Command interface {
	Expect(sections []string) []string
	Run(output io.ReadWriter, args ...string) error
	Help(explain bool) string
}
