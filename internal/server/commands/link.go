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
	"github.com/NHAS/reverse_ssh/pkg/table"
)

type link struct {
}

func (l *link) Run(tty io.ReadWriter, args ...string) error {
	flags, _ := parseFlags(args...)

	if isSet("h", flags) {
		return errors.New(l.Help(false))
	}

	if toList, ok := flags["l"]; ok {
		t, _ := table.NewTable("Active Files", "ID", "GOOS", "GOARCH", "Expires")

		files, err := webserver.List(strings.Join(toList, " "))
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

	if toRemove, ok := flags["r"]; ok {
		for _, id := range toRemove {
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
	if lifetime, ok := flags["t"]; ok {
		if len(lifetime) != 1 {
			return fmt.Errorf("Time supplied %d arguments, expected 1", len(lifetime))
		}

		mins, err := strconv.Atoi(lifetime[0])
		if err != nil {
			return fmt.Errorf("Unable to parse number of minutes (-t): %s", lifetime[0])
		}

		e = time.Duration(mins) * time.Minute
	}

	var homeserver_address string
	if cb, ok := flags["s"]; ok {
		if len(cb) != 1 {
			return fmt.Errorf("Homeserver connect back address supplied %d arguments, expected 1", len(cb))
		}

		homeserver_address = cb[0]

	}

	var goos string
	if cb, ok := flags["goos"]; ok {
		if len(cb) != 1 {
			return fmt.Errorf("GOOS supplied %d arguments, expected 1", len(cb))
		}

		goos = cb[0]

	}

	var goarch string
	if cb, ok := flags["goarch"]; ok {
		if len(cb) != 1 {
			return fmt.Errorf("GOARCH supplied %d arguments, expected 1", len(cb))
		}

		goarch = cb[0]

	}

	url, err := webserver.Build(e, goos, goarch, homeserver_address)
	if err != nil {
		return err
	}

	fmt.Fprintln(tty, url)

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
