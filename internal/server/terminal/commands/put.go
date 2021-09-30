package commands

import (
	"fmt"
	"sync"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal/commands/constants"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

type put struct {
	controllableClients *sync.Map
	log                 logger.Logger
}

func (p *put) Run(term *terminal.Terminal, args ...string) error {

	if len(args) != 1 {
		return fmt.Errorf(p.Help(false))
	}

	return nil
}

func (p *put) Expect(sections []string) []string {

	if len(sections) == 1 {
		return []string{constants.RemoteId}
	}

	return nil
}

func (p *put) Help(explain bool) string {
	if explain {
		return "End a remote controllable host instance."
	}

	return makeHelpText(
		"put <remote_id>",
	)
}

func Put(controllableClients *sync.Map, log logger.Logger) *put {
	return &put{
		controllableClients,
		log,
	}
}
