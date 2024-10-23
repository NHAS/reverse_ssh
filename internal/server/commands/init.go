package commands

import (
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

// This is used for help, so we can generate the nice table
// I would prefer if we could do some sort of autoregistration process for these
var allCommands = map[string]terminal.Command{
	"ls":           &list{},
	"help":         &help{},
	"kill":         &kill{},
	"connect":      &connect{},
	"exit":         &exit{},
	"link":         &link{},
	"exec":         &exec{},
	"who":          &who{},
	"watch":        &watch{},
	"listen":       &listen{},
	"webhook":      &webhook{},
	"version":      &version{},
	"priv":         &privilege{},
	"access":       &access{},
	"autocomplete": &shellAutocomplete{},
	"log":          &logCommand{},
	"clear":        &clear{},
}

func CreateCommands(session string, user *users.User, log logger.Logger, datadir string) map[string]terminal.Command {

	var o = map[string]terminal.Command{
		"ls":           &list{},
		"help":         &help{},
		"kill":         Kill(log),
		"connect":      Connect(session, user, log),
		"exit":         &exit{},
		"link":         &link{},
		"exec":         &exec{},
		"who":          &who{},
		"watch":        Watch(datadir),
		"listen":       Listen(log),
		"webhook":      &webhook{},
		"version":      &version{},
		"priv":         &privilege{},
		"access":       &access{},
		"autocomplete": &shellAutocomplete{},
		"log":          Log(log),
		"clear":        &clear{},
	}

	return o
}

func addDuplicateFlags(helpText string, m map[string]string, flags ...string) {
	for _, flag := range flags {
		m[flag] = helpText
	}
}
