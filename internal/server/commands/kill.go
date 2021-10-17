package commands

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/NHAS/reverse_ssh/internal/server/commands/constants"
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

	isClosed := make(chan bool, 1)

	//Essentially a timeout function for killing the client.
	go func(conn ssh.Conn) {
		//Just in case a malicious client is told to die, then exactly times a 5 second wait inorder to force double close.
		//Frankly, such a small chance of this happening. But meh
		defer func() {
			if r := recover(); r != nil {
				k.Info("Client double closed.")
			}
		}()

		select {
		case <-time.After(2 * time.Second):
			k.Warning("Client failed to exit")
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

	return err
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

	killClient(k.controllableClients, k.log, args[0])

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
		"kill all",
	)
}

func Kill(controllableClients *sync.Map, log logger.Logger) *kill {
	return &kill{
		controllableClients,
		log,
	}
}
