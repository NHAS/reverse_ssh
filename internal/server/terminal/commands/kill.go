package commands

import (
	"fmt"
	"sync"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal/commands/constants"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type kill struct {
	controllableClients *sync.Map
	log                 logger.Logger
}

func (k *kill) Run(term *terminal.Terminal, args ...string) error {

	if len(args) != 1 {
		return fmt.Errorf(k.Help(false))
	}

	cc, ok := k.controllableClients.Load(args[0])
	if !ok {
		return fmt.Errorf("unknown connection host")
	}

	controlClient := cc.(ssh.Conn)

	err := controlClient.Close()
	if err != nil {
		k.log.Error("creating session failed: %s", err)
		return err
	}

	k.controllableClients.Delete(args[0])
	return fmt.Errorf("connection has been killed")
}

func (k *kill) Expect(sections []string) []string {

	if len(sections) == 1 {
		return []string{constants.RemoteId}
	}

	return nil
}

func (k *kill) Help(explain bool) string {
	if explain {
		return "End a remote controllable host instance."
	}

	return makeHelpText(
		"kill <remote_id>",
	)
}

func Kill(controllableClients *sync.Map, log logger.Logger) *kill {
	return &kill{
		controllableClients,
		log,
	}
}
