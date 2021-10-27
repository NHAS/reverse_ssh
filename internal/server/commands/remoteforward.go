package commands

import (
	"fmt"
	"io"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/commands/constants"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type remoteForward struct {
	controllableClients *sync.Map
	log                 logger.Logger
	user                *internal.User
}

func (rf *remoteForward) Run(tty io.ReadWriter, args ...string) error {
	if len(args) != 1 {
		return fmt.Errorf(rf.Help(false))
	}

	if args[0] == "all" {

		internal.EnableForwarding(rf.user.IdString, "all")

		rf.controllableClients.Range(func(idStr interface{}, value interface{}) bool {
			id := idStr.(string)
			cc, ok := rf.controllableClients.Load(id)
			if !ok {
				fmt.Fprintf(tty, "Unknown client %s", id)
			}

			clientConnection := cc.(ssh.Conn)

			for forward, _ := range rf.user.SupportedRemoteForwards {
				_, _, err := clientConnection.SendRequest("tcpip-forward", true, ssh.Marshal(&forward))
				if err != nil {
					fmt.Fprintf(tty, "Unable to start remote forward on %s:%s:%d because %s", id, forward.BindAddr, forward.BindPort, err.Error())
					continue
				}

			}

			return true
		})
		return fmt.Errorf(" connections killed")
	}

	for _, id := range args {

		cc, ok := rf.controllableClients.Load(id)
		if !ok {
			fmt.Fprintf(tty, "Unknown client %s", id)
		}

		clientConnection := cc.(ssh.Conn)

		for forward, _ := range rf.user.SupportedRemoteForwards {
			_, _, err := clientConnection.SendRequest("tcpip-forward", true, ssh.Marshal(&forward))
			if err != nil {
				fmt.Fprintf(tty, "Unable to start remote forward on %s:%s:%d because %s", id, forward.BindAddr, forward.BindPort, err.Error())
			}
		}
	}

	internal.EnableForwarding(rf.user.IdString, args...)

	return nil
}

func (rf *remoteForward) Expect(sections []string) []string {

	if len(sections) == 1 {
		return []string{constants.RemoteId}
	}

	return nil
}

func (rf *remoteForward) Help(explain bool) string {
	if explain {
		return "Enable remote forwarding for specific clients, or all connected clients (will open a remote port and forward all traffic to your device)"
	}

	return makeHelpText(
		"rforward <remote_id>",
		"rforward all",
	)
}

func RemoteForward(user *internal.User, controllableClients *sync.Map, log logger.Logger) *remoteForward {
	return &remoteForward{
		controllableClients,
		log,
		user,
	}
}
