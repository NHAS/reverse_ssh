package commands

import (
	"fmt"
	"io"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

type kill struct {
	log logger.Logger
}

func (k *kill) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {

	if len(line.Arguments) != 1 {
		return fmt.Errorf(k.Help(false))
	}

	connections, err := user.SearchClients(line.Arguments[0].Value())
	if err != nil {
		return err
	}

	if len(connections) == 0 {
		return fmt.Errorf("No clients matched '%s'", line.Arguments[0].Value())
	}

	killedClients := 0
	for id, serverConn := range connections {
		serverConn.SendRequest("kill", false, nil)

		if len(connections) == 1 {
			return fmt.Errorf("%s killed", id)
		}
		killedClients++
	}

	return fmt.Errorf("%d connections killed", killedClients)
}

func (k *kill) Expect(line terminal.ParsedLine) []string {
	if len(line.Arguments) <= 1 {
		return []string{autocomplete.RemoteId}
	}
	return nil
}

func (k *kill) Help(explain bool) string {
	if explain {
		return "Stop the execute of the rssh client."
	}

	return terminal.MakeHelpText(
		"kill <remote_id>",
		"kill <glob pattern>",
		"Stop the execute of the rssh client.",
	)
}

func Kill(log logger.Logger) *kill {
	return &kill{
		log: log,
	}
}
