package commands

import (
	"fmt"
	"io"

	"github.com/NHAS/reverse_ssh/internal/server/clients"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

type kill struct {
	log logger.Logger
}

func (k *kill) Run(tty io.ReadWriter, line terminal.ParsedLine) error {

	if len(line.Leftovers) != 1 {
		return fmt.Errorf(k.Help(false))
	}

	if line.Leftovers[0].Value() == "all" {
		killedClients := 0
		allClients := clients.GetAll()
		for _, v := range allClients {
			v.SendRequest("kill", false, nil)
			killedClients++
		}
		return fmt.Errorf("%d connections killed", killedClients)
	}

	conn, err := clients.Get(line.Leftovers[0].Value())
	if err != nil {
		return err
	}

	_, _, err = conn.SendRequest("kill", false, nil)

	return err
}

func (k *kill) Expect(line terminal.ParsedLine) []string {

	return nil
}

func (k *kill) Help(explain bool) string {
	if explain {
		return "End a remote controllable host instance."
	}

	return makeHelpText(
		"kill <remote_id>",
		"kill all",
	)
}

func Kill(log logger.Logger) *kill {
	return &kill{
		log: log,
	}
}
