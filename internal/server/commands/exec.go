package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/server/clients"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
	"golang.org/x/crypto/ssh"
)

type exec struct {
}

func (e *exec) Run(tty io.ReadWriter, line terminal.ParsedLine) error {
	if terminal.IsSet("h", line.Flags) {
		fmt.Fprintf(tty, "%s", e.Help(false))
		return nil
	}

	filter := ""
	command := ""
	if len(line.LeftoversStrings()) > 0 {
		if len(line.Leftovers) < 2 {
			return fmt.Errorf("Not enough arguments supplied. Needs at least, host|filter command...")
		}

		filter = line.LeftoversStrings()[0]
		command = line.RawLine[line.Leftovers[0].End():]
	} else if len(line.FlagsOrdered) > 0 {
		args := line.FlagsOrdered[len(line.FlagsOrdered)-1].Args
		if len(args) < 2 {
			return fmt.Errorf("Not enough arguments supplied. Needs at least, host|filter command...")
		}

		filter = args[0].Value()
		command = line.RawLine[args[0].End():]
	}

	command = strings.TrimSpace(command)

	matchingClients, err := clients.Search(filter)
	if err != nil {
		return err
	}

	if len(matchingClients) == 0 {
		return fmt.Errorf("Unable to find match for '" + filter + "'\n")
	}

	if !(terminal.IsSet("q", line.Flags) || terminal.IsSet("raw", line.Flags)) {
		fmt.Fprintln(tty, "Effects:")
		count := 0
		for id, client := range matchingClients {
			if count > 5 {
				break
			}

			fmt.Fprintf(tty, "%s (%s)\n", id, client.User()+"@"+client.RemoteAddr().String())
			count++
		}

		if count > 5 {
			fmt.Fprintf(tty, "... %d hosts omitted ...\n", len(matchingClients)-count)
		}

		if !terminal.IsSet("y", line.Flags) {

			fmt.Fprintf(tty, "Run command? [N/y] ")

			if term, ok := tty.(*terminal.Terminal); ok {
				term.EnableRaw()
			}

			b := make([]byte, 1)
			_, err := tty.Read(b)
			if err != nil {
				if term, ok := tty.(*terminal.Terminal); ok {
					term.DisableRaw()
				}
				return err
			}
			if term, ok := tty.(*terminal.Terminal); ok {
				term.DisableRaw()
			}

			if !(b[0] == 'y' || b[0] == 'Y') {
				return fmt.Errorf("\nUser did not enter y/Y, aborting")
			}
		}
	}

	var c struct {
		Cmd string
	}
	c.Cmd = command

	commandByte := ssh.Marshal(&c)

	for id, client := range matchingClients {

		if !(terminal.IsSet("q", line.Flags) || terminal.IsSet("raw", line.Flags)) {
			fmt.Fprint(tty, "\n\n")
			fmt.Fprintf(tty, "%s (%s) output:\n", id, client.User()+"@"+client.RemoteAddr().String())
		}

		newChan, r, err := client.OpenChannel("session", nil)
		if err != nil && !terminal.IsSet("q", line.Flags) {
			fmt.Fprintf(tty, "Failed: %s\n", err)
			continue
		}
		go ssh.DiscardRequests(r)
		defer newChan.Close()

		response, err := newChan.SendRequest("exec", true, commandByte)
		if err != nil && !terminal.IsSet("q", line.Flags) {
			fmt.Fprintf(tty, "Failed: %s\n", err)
			continue
		}

		if !response && !terminal.IsSet("q", line.Flags) {
			fmt.Fprintf(tty, "Failed: client refused\n")
			continue
		}

		if terminal.IsSet("q", line.Flags) {
			io.Copy(io.Discard, newChan)
			continue
		}

		io.Copy(tty, newChan)
	}

	return nil
}

func (e *exec) Expect(line terminal.ParsedLine) []string {

	if line.Focus == nil {
		return []string{autocomplete.RemoteId}
	}

	return nil
}

func (e *exec) Help(explain bool) string {
	if explain {
		return "Execute a command on one or more rssh client"
	}

	return makeHelpText(
		"exec [OPTIONS] filter|host command",
		"Filter uses glob matching against all attributes of a target (hostname, ip, id), allowing you to run a command against multiple machines",
		"\t-q\tQuiet, no output (will also remove confirmation prompt)",
		"\t-y\tNo confirmation prompt",
		"\t--raw\tDo not label output blocks with the client they came from",
	)
}
