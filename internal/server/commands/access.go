package commands

import (
	"errors"
	"fmt"
	"io"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
)

type access struct {
}

func (s *access) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {

	var err error

	pattern, err := line.GetArgString("p")
	if err != nil {
		if err != terminal.ErrFlagNotSet {
			return err
		}
		pattern, err = line.GetArgString("pattern")
		if err != nil && err != terminal.ErrFlagNotSet {
			return err
		}

	}

	newOwners, err := line.GetArgString("o")
	if err != nil {
		if err != terminal.ErrFlagNotSet {
			return err
		}
		newOwners, err = line.GetArgString("owners")
		if err != nil && err != terminal.ErrFlagNotSet {
			return err
		}

	}

	if line.IsSet("c") || line.IsSet("current") {
		newOwners = user.Username()
	}

	if line.IsSet("a") || line.IsSet("all") {
		newOwners = ""
	}

	if spaceMatcher.MatchString(newOwners) {
		return errors.New("new owners cannot contain spaces")
	}

	connections, err := user.SearchClients(pattern)
	if err != nil {
		return err
	}

	if len(connections) == 0 {
		return fmt.Errorf("No clients matched '%s'", pattern)
	}

	if !line.IsSet("y") {
		fmt.Fprintf(tty, "Modifing ownership of %d clients? [N/y] ", len(connections))

		if term, ok := tty.(*terminal.Terminal); ok {
			term.EnableRaw()
		}

		b := make([]byte, 1)
		_, err := tty.Read(b)
		if err != nil {
			if term, ok := tty.(*terminal.Terminal); ok {
				term.DisableRaw(false)
			}
			return err
		}
		if term, ok := tty.(*terminal.Terminal); ok {
			term.DisableRaw(false)
		}

		if !(b[0] == 'y' || b[0] == 'Y') {
			return fmt.Errorf("\nUser did not enter y/Y, aborting")
		}
	}

	changes := 0
	for id := range connections {
		err := user.SetOwnership(id, newOwners)
		if err != nil {
			fmt.Fprintf(tty, "error changing ownership of %s: err %s", id, err)
			continue
		}
		changes++
	}

	return fmt.Errorf("\n%d client owners modified", changes)
}

func (s *access) ValidArgs() map[string]string {

	r := map[string]string{
		"y": "Auto confirm prompt",
	}

	addDuplicateFlags("Clients to act on", r, "p", "pattern")
	addDuplicateFlags("Set the ownership of the client, comma seperated user list", r, "o", "owners")
	addDuplicateFlags("Set the ownership to only the current user", r, "c", "current")
	addDuplicateFlags("Set the ownership to anyone on the server", r, "a", "all")

	return r
}

func (s *access) Expect(line terminal.ParsedLine) []string {
	if line.Section != nil {
		switch line.Section.Value() {
		case "p", "pattern":
			return []string{autocomplete.RemoteId}
		}
	}
	return nil
}

func (s *access) Help(explain bool) string {
	if explain {
		return "Temporarily share/unhide client connection."
	}

	return terminal.MakeHelpText(s.ValidArgs(),
		"access [OPTIONS] -p <FILTER>",
		"Change ownership of client connection, only lasts until restart of rssh server, to make permanent edit authorized_controllee_keys 'owner' option",
		"Filter uses glob matching against all attributes of a target (id, public key hash, hostname, ip)",
	)
}
