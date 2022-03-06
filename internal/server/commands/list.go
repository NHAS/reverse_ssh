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

type List struct {
}

type displayItem struct {
	sc ssh.Conn
	id string
}

func fancyTable(tty io.ReadWriter, applicable []displayItem) {

	t, _ := table.NewTable("Targets", "ID", "Hostname", "IP Address")
	for _, a := range applicable {
		t.AddValues(a.id, a.sc.User(), a.sc.RemoteAddr().String())
	}

	t.Fprint(tty)
}

func (l *List) Run(tty io.ReadWriter, line terminal.ParsedLine) error {

	filter := ""
	if len(line.LeftoversStrings()) > 0 {
		filter = strings.Join(line.LeftoversStrings(), " ")
	} else if len(line.FlagsOrdered) > 1 {
		args := line.FlagsOrdered[len(line.FlagsOrdered)-1].Args
		if len(args) != 0 {
			filter = line.RawLine[args[0].End():]
		}
	}

	if terminal.IsSet("h", line.Flags) {
		fmt.Fprintf(tty, "%s", l.Help(false))
		return nil
	}

	var toReturn []displayItem

	matchingClients, err := clients.Search(filter)
	if err != nil {
		return err
	}

	if len(matchingClients) == 0 {
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

	if terminal.IsSet("t", line.Flags) {
		fancyTable(tty, toReturn)
		return nil
	}

	sep := "\n"

	for i, tr := range toReturn {

		fmt.Fprintf(tty, "%s %s %s", tr.id, tr.sc.User(), tr.sc.RemoteAddr().String())

		if i != len(toReturn)-1 {
			fmt.Fprint(tty, sep)
		}
	}

	fmt.Fprint(tty, "\n")

	return nil
}

func (l *List) Expect(line terminal.ParsedLine) []string {
	if len(line.Leftovers) <= 1 {
		return []string{autocomplete.RemoteId}
	}
	return nil
}

func (l *List) Help(explain bool) string {
	if explain {
		return "List connected controllable hosts."
	}

	return makeHelpText(
		"ls [OPTION] [FILTER]",
		"Filter uses glob matching against all attributes of a target (hostname, ip, id)",
		"\t-t\tPrint all attributes in pretty table",
		"\t-h\tPrint help",
	)
}
