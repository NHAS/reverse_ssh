package commands

import (
	"io"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type Command interface {
	Run(output io.ReadWriter, args ...string) error
	Help(explain bool) string
}

//This is used for help, so we can generate the nice table
// I would prefer if we could do some sort of autoregistration process for these
var allCommands = map[string]Command{
	"ls":   &List{},
	"help": &help{},
	"kill": &kill{},
}

func CreateCommands(user *internal.User,
	connection ssh.Channel,
	requests <-chan *ssh.Request,
	log logger.Logger) map[string]Command {

	var o = map[string]Command{
		"ls":   &List{},
		"help": Help(),
		"kill": Kill(log),
	}

	return o
}
