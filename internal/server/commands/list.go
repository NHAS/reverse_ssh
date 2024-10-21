package commands

import (
	"fmt"
	"io"
	"log"
	"sort"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
	"github.com/NHAS/reverse_ssh/pkg/table"
	"github.com/fatih/color"
	"golang.org/x/crypto/ssh"
)

type list struct {
}

type displayItem struct {
	sc ssh.ServerConn
	id string
}

func fancyTable(tty io.ReadWriter, applicable []displayItem) {

	t, _ := table.NewTable("Targets", "IDs", "Owners", "Version")
	for _, a := range applicable {

		keyId := a.sc.Permissions.Extensions["pubkey-fp"]
		if a.sc.Permissions.Extensions["comment"] != "" {
			keyId = a.sc.Permissions.Extensions["comment"]
		}

		owners := a.sc.Permissions.Extensions["owners"]
		if owners == "" {
			owners = "public"
		} else {
			owners = strings.Join(strings.Split(a.sc.Permissions.Extensions["owners"], ","), "\n")
		}

		if err := t.AddValues(fmt.Sprintf("%s\n%s\n%s\n%s\n", a.id, keyId, users.NormaliseHostname(a.sc.User()), a.sc.RemoteAddr().String()), owners, string(a.sc.ClientVersion())); err != nil {
			log.Println("Error drawing pretty ls table (THIS IS A BUG): ", err)
			return
		}
	}

	t.Fprint(tty)
}

func (l *list) ValidArgs() map[string]string {
	return map[string]string{
		"t": "Print all attributes in pretty table",
		"h": "Print help"}
}

func (l *list) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {

	filter := ""
	if len(line.ArgumentsAsStrings()) > 0 {
		filter = strings.Join(line.ArgumentsAsStrings(), " ")
	} else if len(line.FlagsOrdered) > 1 {
		args := line.FlagsOrdered[len(line.FlagsOrdered)-1].Args
		if len(args) != 0 {
			filter = line.RawLine[args[0].End():]
		}
	}

	var toReturn []displayItem

	matchingClients, err := user.SearchClients(filter)
	if err != nil {
		return err
	}

	if len(matchingClients) == 0 {
		if len(filter) == 0 {
			return fmt.Errorf("No RSSH clients connected")
		}

		return fmt.Errorf("Unable to find match for '" + filter + "'")
	}

	ids := []string{}
	for id := range matchingClients {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	for _, id := range ids {
		toReturn = append(toReturn, displayItem{id: id, sc: *matchingClients[id]})
	}

	if line.IsSet("t") {
		fancyTable(tty, toReturn)
		return nil
	}

	sep := "\n"

	for i, tr := range toReturn {

		keyId := tr.sc.Permissions.Extensions["pubkey-fp"]
		if tr.sc.Permissions.Extensions["comment"] != "" {
			keyId = tr.sc.Permissions.Extensions["comment"]
		}

		owners := tr.sc.Permissions.Extensions["owners"]
		if owners == "" {
			owners = "public"
		}

		fmt.Fprintf(tty, "%s %s %s %s, owners: %s, version: %s", color.YellowString(tr.id), keyId, color.BlueString(users.NormaliseHostname(tr.sc.User())), tr.sc.RemoteAddr().String(), owners, tr.sc.ClientVersion())

		if i != len(toReturn)-1 {
			fmt.Fprint(tty, sep)
		}
	}

	fmt.Fprint(tty, "\n")

	return nil
}

func (l *list) Expect(line terminal.ParsedLine) []string {
	if len(line.Arguments) <= 1 {
		return []string{autocomplete.RemoteId}
	}
	return nil
}

func (l *list) Help(explain bool) string {
	if explain {
		return "List connected controllable hosts."
	}

	return terminal.MakeHelpText(l.ValidArgs(),
		"ls [OPTION] [FILTER]",
		"Filter uses glob matching against all attributes of a target (id, public key hash, hostname, ip)",
	)
}
