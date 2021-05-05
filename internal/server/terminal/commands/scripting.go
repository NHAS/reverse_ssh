package commands

import (
	"fmt"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/trie"
)

type scripting struct {
	modeAutoComplete    *trie.Trie
	user                *users.User
	controllableClients *sync.Map
}

func (s *scripting) Run(term *terminal.Terminal, args ...string) error {
	if len(args) < 1 {
		helpText := "rc enable <remote_id> <rc file path>\n"
		helpText += "rc disable <remote_id> <rc file path>\n"
		helpText += "rc ls <remote_id>\n"
		return fmt.Errorf(helpText)
	}

	switch args[0] {

	case "enable", "disable":
		if len(args) != 3 {
			return fmt.Errorf("Not enough args for rc %s. rc %s <remote_id> <rc file path>", args[0], args[0])
		}

		if _, ok := s.controllableClients.Load(args[1]); !ok {
			return fmt.Errorf("Unknown remote id")
		}

		currentHostRCFiles, ok := s.user.EnabledRcfiles[args[1]]
		if !ok {
			currentHostRCFiles = []string{}
		}

		index := -1
		for i := 0; i < len(currentHostRCFiles); i++ {
			if currentHostRCFiles[i] == args[2] {
				index = i
				break
			}
		}

		if args[0] == "enable" && index == -1 {
			if !internal.FileExists(args[2]) {
				return fmt.Errorf("File %s does not exist", args[2])
			}

			s.user.EnabledRcfiles[args[1]] = append(currentHostRCFiles, args[2])

			fmt.Fprintf(term, "Host %s rc files\n", args[1])
			for _, v := range s.user.EnabledRcfiles[args[1]] {
				fmt.Fprintf(term, "\t%s\n", v)
			}
		}

		if args[0] == "disable" {
			if index != -1 {
				currentHostRCFiles[index] = currentHostRCFiles[len(currentHostRCFiles)-1]
				s.user.EnabledRcfiles[args[1]] = currentHostRCFiles[:len(currentHostRCFiles)-1]
				fmt.Fprintf(term, "Disabled %s for %s\n", args[2], args[1])
				return nil
			}
			fmt.Fprintf(term, "%s did not have %s enabled\n", args[1], args[2])
		}

	case "ls":
		if len(args) == 2 {

			rcfiles, ok := s.user.EnabledRcfiles[args[1]]
			if !ok {
				return fmt.Errorf("Host [%s] doesnt have any Rc files set", args[1])
			}

			for _, v := range rcfiles {
				fmt.Fprintf(term, "%s\n", v)
			}

			return nil
		}

		for k, v := range s.user.EnabledRcfiles {
			fmt.Fprintf(term, "%s\n", k)
			for _, rcfile := range v {
				fmt.Fprintf(term, "\t%s\n", rcfile)
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
			return []string{RemoteId}
		}

		if len(sections) == 3 {
			currentHostRCFiles, ok := s.user.EnabledRcfiles[sections[1]]
			if ok {
				return trie.NewTrie(currentHostRCFiles...).PrefixMatch(sections[2])
			}
		}

	case "ls":
		return []string{RemoteId}

	}

	return nil
}

func RC(user *users.User, controllableClients *sync.Map) *scripting {
	return &scripting{
		modeAutoComplete:    trie.NewTrie("enable", "disable", "ls"),
		controllableClients: controllableClients,
		user:                user,
	}
}
