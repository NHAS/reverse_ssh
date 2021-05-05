package terminal

type Base interface {
	Expect(sections []string) []string
	Run(term *Terminal, args ...string) error
	Help(brief bool) string
}
