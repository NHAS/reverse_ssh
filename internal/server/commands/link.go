package commands

import (
	"io"
)

type link struct {
}

func (l *link) Run(tty io.ReadWriter, args ...string) error {

	return nil
}

func (l *link) Expect(sections []string) []string {
	return nil
}

func (e *link) Help(explain bool) string {
	if explain {
		return "Generate client binary and return link to it"
	}

	return makeHelpText(
		"link [OPTIONS]",
		"Link will compile a client and serve the resulting binary on a link which is returned.",
		"This requires the web server component has been enabled.",
		"\t-t\tSet number of minutes link exists for (default is one time use)",
		"\t-s\tRandom link size, setting this too short may end up clobbering other download links (default length 16)",
		"\t-h\tSet homeserver address, defaults to server --homeserver_address if set, or server listen address if not.",
	)
}

func Link() *link {
	return &link{}
}
