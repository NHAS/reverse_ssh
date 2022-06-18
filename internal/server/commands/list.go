package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/server/clients"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
	"github.com/NHAS/reverse_ssh/pkg/table"
	"golang.org/x/crypto/ssh"
)

type list struct {
}

type displayItem struct {
	sc ssh.Conn
	id string
}

func fancyTable(tty io.ReadWriter, applicable []displayItem) {

	t, _ := table.NewTable("Targets", "ID", "Hostname", "IP Address", "Version")
	for _, a := range applicable {
		t.AddValues(a.id, clients.NormaliseHostname(a.sc.User()), a.sc.RemoteAddr().String(), string(a.sc.ClientVersion()))
	}

	t.Fprint(tty)
}

func (l *list) Run(tty io.ReadWriter, line terminal.ParsedLine) error {

	filter := ""
	if len(line.ArgumentsAsStrings()) > 0 {
		filter = strings.Join(line.ArgumentsAsStrings(), " ")
	} else if len(line.FlagsOrdered) > 1 {
		args := line.FlagsOrdered[len(line.FlagsOrdered)-1].Args
		if len(args) != 0 {
			filter = line.RawLine[args[0].End():]
		}
	}

	if line.IsSet("h") {
		fmt.Fprintf(tty, "%s", l.Help(false))
		return nil
	}

	var toReturn []displayItem

	matchingClients, err := clients.Search(filter)
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
		toReturn = append(toReturn, displayItem{id: id, sc: matchingClients[id]})
	}

	if line.IsSet("t") {
		fancyTable(tty, toReturn)
		return nil
	}

	sep := "\n"

	for i, tr := range toReturn {

		fmt.Fprintf(tty, "%s %s %s, version: %s", tr.id, clients.NormaliseHostname(tr.sc.User()), tr.sc.RemoteAddr().String(), tr.sc.ClientVersion())

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

	return terminal.MakeHelpText(
		"ls [OPTION] [FILTER]",
		"Filter uses glob matching against all attributes of a target (hostname, ip, id)",
		"\t-t\tPrint all attributes in pretty table",
		"\t-h\tPrint help",
	)
}
