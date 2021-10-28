package commands

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/NHAS/reverse_ssh/internal/server/clients"
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

	filter := ""
	flags := map[byte]bool{}
	for _, arg := range args {
		if len(arg) > 0 && arg[0] == '-' {
			for _, c := range arg[1:] {
				flags[byte(c)] = true
			}

			continue
		}

		filter = arg
	}

	if flags['h'] {
		fmt.Fprintf(tty, "%s", l.Help(false))
		return nil
	}

	_, err := filepath.Match(filter, "")
	if err != nil {
		return fmt.Errorf("Filter is not well formed")
	}

	var toReturn []displayItem

	clients := clients.GetAll()
	for id, conn := range clients {

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

	if flags['t'] {
		fancyTable(tty, toReturn)
		return nil
	}

	sep := ", "
	if flags['l'] {
		sep = "\n"
	}

	for i, tr := range toReturn {

		if !flags['n'] && !flags['i'] && !flags['a'] {
			fmt.Fprint(tty, tr.id)
			if i != len(toReturn) {
				fmt.Fprint(tty, sep)
			}
			continue
		}

		if flags['a'] {
			fmt.Fprint(tty, tr.id)
		}

		if flags['n'] || flags['a'] {
			fmt.Fprint(tty, " "+tr.sc.User())
		}

		if flags['i'] || flags['a'] {
			fmt.Fprint(tty, " "+tr.sc.RemoteAddr().String())
		}

		if i != len(toReturn) {
			fmt.Fprint(tty, sep)
		}
	}

	if !flags['l'] {
		fmt.Fprint(tty, "\n")
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
