package commands

import (
	"fmt"
	"io"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/commands/constants"
	"github.com/NHAS/reverse_ssh/pkg/trie"
)

type scripting struct {
	modeAutoComplete    *trie.Trie
	user                *internal.User
	controllableClients *sync.Map
}

func (s *scripting) enable(tty io.ReadWriter, remoteid, rcfile string) error {
	if !internal.FileExists(rcfile) {
		return fmt.Errorf("File %s does not exist", rcfile)
	}

	currentHostRCFiles, ok := s.user.EnabledRcfiles[remoteid]
	if !ok {
		currentHostRCFiles = []string{}
	}

	for _, v := range currentHostRCFiles {
		if v == rcfile {
			fmt.Fprintf(tty, "%s is already enabled for %s\n", rcfile, remoteid)
			return nil // Already exists so just exit!
		}
	}

	s.user.EnabledRcfiles[remoteid] = append(currentHostRCFiles, rcfile)

	fmt.Fprintf(tty, "Host %s rc files\n", remoteid)
	for _, v := range s.user.EnabledRcfiles[remoteid] {
		fmt.Fprintf(tty, "\t%s\n", v)
	}

	return nil
}

func (s *scripting) disable(remoteid, rcfile string) error {
	currentHostRCFiles, ok := s.user.EnabledRcfiles[remoteid]
	if !ok {

		return fmt.Errorf("Host %s has no rc files\n", remoteid)
	}

	index := -1
	for i := 0; i < len(currentHostRCFiles); i++ {
		if currentHostRCFiles[i] == rcfile {
			index = i
			break
		}
	}

	if index != -1 {
		currentHostRCFiles[index] = currentHostRCFiles[len(currentHostRCFiles)-1]
		s.user.EnabledRcfiles[remoteid] = currentHostRCFiles[:len(currentHostRCFiles)-1]

		return fmt.Errorf("Disabled %s for %s\n", rcfile, remoteid)
	}

	return fmt.Errorf("%s did not have %s enabled\n", remoteid, rcfile)

}

func (s *scripting) Run(tty io.ReadWriter, args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf(s.Help(false))
	}

	switch args[0] {

	case "enable", "disable":
		if len(args) != 3 {
			return fmt.Errorf("Not enough args for rc %s. rc %s <remote_id> <rc file path>", args[0], args[0])
		}

		if _, ok := s.controllableClients.Load(args[1]); !ok {
			return fmt.Errorf("Unknown remote id")
		}

		if args[0] == "enable" {
			return s.enable(tty, args[1], args[2])
		}

		if args[0] == "disable" {
			return s.disable(args[1], args[2])
		}

	case "ls":
		if len(args) == 2 {

			rcfiles, ok := s.user.EnabledRcfiles[args[1]]
			if !ok {
				return fmt.Errorf("Host [%s] doesnt have any Rc files set", args[1])
			}

			for _, v := range rcfiles {
				fmt.Fprintf(tty, "%s\n", v)
			}

			return nil
		}

		for k, v := range s.user.EnabledRcfiles {
			fmt.Fprintf(tty, "%s\n", k)
			for _, rcfile := range v {
				fmt.Fprintf(tty, "\t%s\n", rcfile)
			}
		}

	default:
		return fmt.Errorf("Unknown rc sub command: %s", args[0])
	}

	return nil
}

func (s *scripting) Expect(sections []string) []string {
	if len(sections) == 1 {
		return s.modeAutoComplete.PrefixMatch(sections[0])
	}

	switch sections[0] {
	case "enable", "disable":
		if len(sections) == 2 {
			return []string{constants.RemoteId}
		}

		if len(sections) == 3 {
			currentHostRCFiles, ok := s.user.EnabledRcfiles[sections[1]]
			if ok {
				return trie.NewTrie(currentHostRCFiles...).PrefixMatch(sections[2])
			}
		}

	case "ls":
		return []string{constants.RemoteId}

	}

	return nil
}

func (s *scripting) Help(explain bool) string {
	if explain {
		return "Set scripts to run on connection to remote host."
	}

	return makeHelpText(
		"rc enable <remote_id> <rc file path>",
		"rc disable <remote_id> <rc file path>",
		"rc ls [remote_id]",
	)
}

func RC(user *internal.User, controllableClients *sync.Map) *scripting {
	return &scripting{
		modeAutoComplete:    trie.NewTrie("enable", "disable", "ls"),
		controllableClients: controllableClients,
		user:                user,
	}
}
