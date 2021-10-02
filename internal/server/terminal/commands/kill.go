package commands

import (
	"fmt"
	"io"
	"sync"
	"time"

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

	isClosed := make(chan bool, 1)

	//Essentially a timeout function for killing the client.
	go func(conn ssh.Conn) {
		//Just in case a malicious client is told to die, then exactly times a 5 second wait inorder to force double close.
		//Frankly, such a small chance of this happening. But meh
		defer func() {
			if r := recover(); r != nil {
				k.log.Info("Client double closed.")
			}
		}()

		select {
		case <-time.After(2 * time.Second):
			k.log.Warning("Client failed to exit")
			controlClient.Close()
		case <-isClosed:
			return
		}

	}(controlClient)

	_, _, err := controlClient.SendRequest("kill", true, nil)
	//If connection was closed, causing WantReply to fail
	if err == io.EOF {
		isClosed <- true
	}

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
