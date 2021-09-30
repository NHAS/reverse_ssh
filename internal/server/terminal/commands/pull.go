package commands

import (
	"fmt"
	"sync"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal/commands/constants"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

type pull struct {
	controllableClients *sync.Map
	log                 logger.Logger
}

func (p *pull) Run(term *terminal.Terminal, args ...string) error {

	if len(args) != 1 {
		return fmt.Errorf(p.Help(false))
	}

	return nil
}

func (p *pull) Expect(sections []string) []string {

	if len(sections) == 1 {
		return []string{constants.RemoteId}
	}

	return nil
}

func (p *pull) Help(explain bool) string {
	if explain {
		return "End a remote controllable host instance."
	}

	return makeHelpText(
		"pull <remote_id> <remote_file> <local_file>",
	)
}

func Pull(controllableClients *sync.Map, log logger.Logger) *pull {
	return &pull{
		controllableClients,
		log,
	}
}
