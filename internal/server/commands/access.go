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
	if len(line.Arguments) != 1 {
		return fmt.Errorf(s.Help(false))
	}

	var err error

	newOwners, err := line.GetArgString("o")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}
	newOwners, err = line.GetArgString("owners")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
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

	connections, err := user.SearchClients(line.Arguments[0].Value())
	if err != nil {
		return err
	}

	if len(connections) == 0 {
		return fmt.Errorf("No clients matched '%s'", line.Arguments[0].Value())
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

	return fmt.Errorf("%d client owners modified", changes)
}

func (s *access) Expect(line terminal.ParsedLine) []string {
	if len(line.Arguments) <= 1 {
		return []string{autocomplete.RemoteId}
	}
	return nil
}

func (s *access) Help(explain bool) string {
	if explain {
		return "Temporarily share/unhide client connection."
	}

	return terminal.MakeHelpText(
		"access [OPTIONS] <FILTER>",
		"Change ownership of client connection, only lasts until restart of rssh server, to make permanent edit authorized_controllee_keys 'owner' option",
		"Filter uses glob matching against all attributes of a target (id, public key hash, hostname, ip)",
		"-o|--owners\tSet the ownership of the client, comma seperated user list",
		"-c|--current\tSet the ownership to only the current user",
		"-a|--all\tSet the ownership to anyone on the server",
	)
}
