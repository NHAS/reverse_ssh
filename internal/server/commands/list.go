package commands

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/server/clients"
	"github.com/NHAS/reverse_ssh/internal/terminal"
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

	filter := strings.Join(line.LeftoversStrings(), " ")

	if terminal.IsSet("h", line.Flags) {
		fmt.Fprintf(tty, "%s", l.Help(false))
		return nil
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

	if terminal.IsSet("t", line.Flags) {
		fancyTable(tty, toReturn)
		return nil
	}

	sep := ", "
	if terminal.IsSet("l", line.Flags) {
		sep = "\n"
	}

	for i, tr := range toReturn {

		if !terminal.IsSet("n", line.Flags) && !terminal.IsSet("i", line.Flags) && !terminal.IsSet("a", line.Flags) {
			fmt.Fprint(tty, tr.id)
			if i != len(toReturn)-1 {
				fmt.Fprint(tty, sep)
			}
			continue
		}

		if terminal.IsSet("a", line.Flags) {
			fmt.Fprint(tty, tr.id)
		}

		if terminal.IsSet("n", line.Flags) || terminal.IsSet("a", line.Flags) {
			fmt.Fprint(tty, " "+tr.sc.User())
		}

		if terminal.IsSet("i", line.Flags) || terminal.IsSet("a", line.Flags) {
			fmt.Fprint(tty, " "+tr.sc.RemoteAddr().String())
		}

		if i != len(toReturn)-1 {
			fmt.Fprint(tty, sep)
		}
	}

	fmt.Fprint(tty, "\n")

	return nil
}

func (l *List) Expect(line terminal.ParsedLine) []string {

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
