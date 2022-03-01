package commands

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/server/clients"
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

func (l *List) Run(tty io.ReadWriter, args ...string) error {

	flags, leftover := parseFlags(args...)
	filter := strings.Join(leftover, " ")

	if isSet("h", flags) {
		fmt.Fprintf(tty, "%s", l.Help(false))
		return nil
	}

	// If we have a single option e.g -a, it can capture the filter so make sure we put it in the right place
	for _, c := range "tlnai" {
		if len(flags[string(c)]) > 0 {
			filter += strings.Join(flags[string(c)], " ")
		}
	}

	_, err := filepath.Match(filter, "")
	if err != nil {
		return fmt.Errorf("Filter is not well formed")
	}

	var toReturn []displayItem

	clients := clients.GetAll()

	ids := []string{}
	for id := range clients {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	for _, id := range ids {

		conn := clients[id]

		if filter == "" {
			toReturn = append(toReturn, displayItem{id: id, sc: conn})
			continue
		}

		match, _ := filepath.Match(filter, id)
		if match {
			toReturn = append(toReturn, displayItem{id: id, sc: conn})
			continue
		}

		match, _ = filepath.Match(filter, conn.User())
		if match {
			toReturn = append(toReturn, displayItem{id: id, sc: conn})
			continue
		}

		match, _ = filepath.Match(filter, conn.RemoteAddr().String())
		if match {
			toReturn = append(toReturn, displayItem{id: id, sc: conn})
			continue
		}
	}

	if isSet("t", flags) {
		fancyTable(tty, toReturn)
		return nil
	}

	sep := ", "
	if isSet("l", flags) {
		sep = "\n"
	}

	for i, tr := range toReturn {

		if !isSet("n", flags) && !isSet("i", flags) && !isSet("a", flags) {
			fmt.Fprint(tty, tr.id)
			if i != len(toReturn)-1 {
				fmt.Fprint(tty, sep)
			}
			continue
		}

		if isSet("a", flags) {
			fmt.Fprint(tty, tr.id)
		}

		if isSet("n", flags) || isSet("a", flags) {
			fmt.Fprint(tty, " "+tr.sc.User())
		}

		if isSet("i", flags) || isSet("a", flags) {
			fmt.Fprint(tty, " "+tr.sc.RemoteAddr().String())
		}

		if i != len(toReturn)-1 {
			fmt.Fprint(tty, sep)
		}
	}

	fmt.Fprint(tty, "\n")

	return nil
}

func (l *List) Expect(sections []string) []string {
	if len(sections) == 1 {
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
		"\t-a\tShow all attributes",
		"\t-n\tShow only hostnames",
		"\t-i\tShow only IP",
		"\t-t\tPrint all attributes in pretty table",
		"\t-l\tPrint with newline rather than space",
		"\t-h\tPrint help",
	)
}
