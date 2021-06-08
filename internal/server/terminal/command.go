package terminal

type Command interface {
	Expect(sections []string) []string
	Run(term *Terminal, args ...string) error
	Help(explain bool) string
}
