package terminal

type Base interface {
	Expect(section int) string
	Run(term *Terminal, args ...string) error
}
