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

	targets := args
	if args[0] == "all" {

		targets = []string{}
		rf.controllableClients.Range(func(idStr interface{}, value interface{}) bool {
			id := idStr.(string)
			targets = append(targets, id)
			return true
		})
	}

	failed := []string{}

	for _, id := range targets {

		cc, ok := rf.controllableClients.Load(id)
		if !ok {
			fmt.Fprintf(tty, "Unknown client %s", id)
			continue
		}

		clientConnection := cc.(ssh.Conn)

		for forward := range rf.user.SupportedRemoteForwards {
			ok, res, err := clientConnection.SendRequest("tcpip-forward", true, ssh.Marshal(&forward))
			if err != nil || !ok {

				reason := string(res)
				if err != nil {
					reason = err.Error()
				}

				failed = append(failed, fmt.Sprintf("%s:%d : Error %s", forward.BindAddr, forward.BindPort, reason))
			}
		}

	}

	userRemoteForwards := len(rf.user.SupportedRemoteForwards)

	fmt.Fprintf(tty, "Requested %d/%d forward/s successfully!\n", userRemoteForwards-len(failed), userRemoteForwards)
	for _, v := range failed {
		fmt.Fprintf(tty, "\t%s\n", v)
	}

	return internal.EnableForwarding(rf.user.IdString, args...)
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
