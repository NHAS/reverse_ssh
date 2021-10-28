package commands

import (
	"fmt"
	"io"

	"github.com/NHAS/reverse_ssh/internal/server/clients"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

type kill struct {
	log logger.Logger
}

func (k *kill) Run(tty io.ReadWriter, args ...string) error {

	if len(args) != 1 {
		return fmt.Errorf(k.Help(false))
	}

	if args[0] == "all" {
		killedClients := 0
		allClients := clients.GetAll()
		for _, v := range allClients {
			v.SendRequest("kill", false, nil)
			killedClients++
		}
		return fmt.Errorf("%d connections killed", killedClients)
	}

	conn, err := clients.Get(args[0])
	if err != nil {
		return err
	}

	_, _, err = conn.SendRequest("kill", false, nil)

	return err
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
