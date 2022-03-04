package commands

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NHAS/reverse_ssh/internal/server/webserver"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
	"github.com/NHAS/reverse_ssh/pkg/table"
)

type link struct {
}

func (l *link) Run(tty io.ReadWriter, line terminal.ParsedLine) error {

	if terminal.IsSet("h", line.Flags) {
		return errors.New(l.Help(false))
	}

	if toList, ok := line.Flags["l"]; ok {
		t, _ := table.NewTable("Active Files", "ID", "GOOS", "GOARCH", "Expires")

		files, err := webserver.List(strings.Join(toList.ArgValues(), " "))
		if err != nil {
			return err
		}

		ids := []string{}
		for id := range files {
			ids = append(ids, id)
		}

		sort.Strings(ids)

		for _, id := range ids {
			file := files[id]

			expiry := "N/A"
			if file.Expiry != 0 {
				expiry = file.Timestamp.Add(file.Expiry).String()
			}
			t.AddValues(id, file.Goos, file.Goarch, expiry)
		}

		t.Fprint(tty)

		return nil

	}

	if toRemove, ok := line.Flags["r"]; ok {
		if len(toRemove.Args) == 0 {
			fmt.Fprintf(tty, "No argument supplied\n")

			return nil
		}

		files, err := webserver.List(strings.Join(toRemove.ArgValues(), " "))
		if err != nil {
			return err
		}

		for id := range files {
			err := webserver.Delete(id)
			if err != nil {
				fmt.Fprintf(tty, "Unable to remove %s: %s\n", id, err)
				continue
			}
			fmt.Fprintln(tty, "Removed", id)
		}

		return nil

	}

	var e time.Duration
	if lifetime, ok := line.Flags["t"]; ok {
		if len(lifetime.Args) != 1 {
			return fmt.Errorf("Time supplied %d arguments, expected 1", len(lifetime.Args))
		}

		mins, err := strconv.Atoi(lifetime.Args[0].Value())
		if err != nil {
			return fmt.Errorf("Unable to parse number of minutes (-t): %s", lifetime.Args[0].Value())
		}

		e = time.Duration(mins) * time.Minute
	}

	var homeserver_address string
	if cb, ok := line.Flags["s"]; ok {
		if len(cb.Args) != 1 {
			return fmt.Errorf("Homeserver connect back address supplied %d arguments, expected 1", len(cb.Args))
		}

		homeserver_address = cb.Args[0].Value()

	}

	var goos string
	if cb, ok := line.Flags["goos"]; ok {
		if len(cb.Args) != 1 {
			return fmt.Errorf("GOOS supplied %d arguments, expected 1", len(cb.Args))
		}

		goos = cb.Args[0].Value()

	}

	var goarch string
	if cb, ok := line.Flags["goarch"]; ok {
		if len(cb.Args) != 1 {
			return fmt.Errorf("GOARCH supplied %d arguments, expected 1", len(cb.Args))
		}

		goarch = cb.Args[0].Value()

	}

	url, err := webserver.Build(e, goos, goarch, homeserver_address)
	if err != nil {
		return err
	}

	fmt.Fprintln(tty, url)

	return nil
}

func (l *link) Expect(line terminal.ParsedLine) []string {
	if line.Section != nil {
		switch line.Section.Value() {
		case "l", "r":
			return []string{autocomplete.WebServerFileIds}
		}
	}

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
		"\t-s\tSet homeserver address, defaults to server --homeserver_address if set, or server listen address if not.",
		"\t-l\tList currently active download links",
		"\t-r\tRemove download link",
		"\t--goos\tSet the target build operating system (default to runtime GOOS)",
		"\t--goarch\tSet the target build architecture (default to runtime GOARCH)",
	)
}

func Link() *link {
	return &link{}
}
