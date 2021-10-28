package commands

import (
	"fmt"
	"io"
	"sync"

	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type kill struct {
	controllableClients *sync.Map
	log                 logger.Logger
}

func killClient(controllableClients *sync.Map, k logger.Logger, id string) error {

	cc, ok := controllableClients.Load(id)
	if !ok {
		return fmt.Errorf("unknown connection host")
	}

	controlClient := cc.(ssh.Conn)

	controlClient.SendRequest("kill", false, nil)

	return nil
}

func (k *kill) Run(tty io.ReadWriter, args ...string) error {

	if len(args) != 1 {
		return fmt.Errorf(k.Help(false))
	}

	if args[0] == "all" {
		killedClients := 0
		k.controllableClients.Range(func(idStr interface{}, value interface{}) bool {
			killClient(k.controllableClients, k.log, idStr.(string))

			killedClients++

			return true
		})
		return fmt.Errorf("%d connections killed", killedClients)
	}

	return killClient(k.controllableClients, k.log, args[0])
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

func Kill(controllableClients *sync.Map, log logger.Logger) *kill {
	return &kill{
		controllableClients,
		log,
	}
}
