package commands

import (
	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

//This is used for help, so we can generate the nice table
// I would prefer if we could do some sort of autoregistration process for these
var allCommands = map[string]terminal.Command{
	"ls":      &List{},
	"help":    &help{},
	"kill":    &kill{},
	"connect": &connect{},
	"exit":    &exit{},
	"link":    &link{},
}

func CreateCommands(user *internal.User, log logger.Logger) map[string]terminal.Command {

	var o = map[string]terminal.Command{
		"ls":      &List{},
		"help":    Help(),
		"kill":    Kill(log),
		"connect": Connect(user, log),
		"exit":    &exit{},
		"link":    &link{},
	}

	return o
}
