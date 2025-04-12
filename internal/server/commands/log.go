package commands

import (
	"fmt"
	"io"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type logCommand struct {
}

func (l *logCommand) ValidArgs() map[string]string {
	return map[string]string{
		"c":          "client to collect logging from",
		"log-level":  "Set client log level, default for generated clients is currently: " + fmt.Sprintf("%q", logger.UrgencyToStr(logger.GetLogLevel())),
		"to-file":    "direct output to file, takes a path as an argument",
		"to-console": "directs output to the server console (or current connection), stop with any keypress",
	}
}

func (l *logCommand) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {

	if !line.IsSet("c") {
		fmt.Fprintln(tty, "missing client -c")
		return nil
	}

	client, err := line.GetArgString("c")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	connection, err := user.GetClient(client)
	if err != nil {
		return err
	}

	logLevel, err := line.GetArgString("log-level")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	} else {

		_, err := logger.StrToUrgency(logLevel)
		if err != nil {
			return fmt.Errorf("invalid log level %q", logLevel)
		}

		_, _, err = connection.SendRequest("log-level", false, []byte(logLevel))
		if err != nil {
			return fmt.Errorf("failed to send log level request to client (may be outdated): %s", err)
		}
	}

	if line.IsSet("to-console") {

		term, isTerm := tty.(*terminal.Terminal)
		if isTerm {
			term.EnableRaw()
		}

		consoleLog, reqs, err := connection.OpenChannel("log-to-console", nil)
		if err != nil {
			return fmt.Errorf("client would not open log to console channel (maybe wrong version): %s", err)
		}

		go ssh.DiscardRequests(reqs)

		go func() {

			b := make([]byte, 1)
			tty.Read(b)

			consoleLog.Close()
		}()

		for {
			buff := make([]byte, 1024)
			n, err := consoleLog.Read(buff)
			if err != nil {
				break
			}

			fmt.Fprintf(tty, "%s\r", buff[:n])
		}

		if isTerm {
			term.DisableRaw(false)
		}

	} else if line.IsSet("to-file") {

		filepath, err := line.GetArgString("to-file")
		if err != nil && err != terminal.ErrFlagNotSet {
			return err
		}

		_, _, err = connection.SendRequest("log-to-file", false, []byte(filepath))
		if err != nil {
			return fmt.Errorf("failed to send request to client: %s", err)
		}
		fmt.Fprintln(tty, "log to file request sent to client!")
	}

	return nil
}

func (l *logCommand) Expect(line terminal.ParsedLine) []string {
	if line.Section != nil {
		switch line.Section.Value() {
		case "c":
			return []string{autocomplete.RemoteId}
		}
	}

	return nil
}

func (l *logCommand) Help(explain bool) string {

	const description = "Collect log output from client"
	if explain {
		return description
	}

	return terminal.MakeHelpText(l.ValidArgs(),
		"log [OPTIONS] <remote_id>",
		description,
	)
}

func Log(log logger.Logger) *logCommand {
	return &logCommand{}
}
