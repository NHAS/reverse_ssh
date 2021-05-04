package commands

import (
	"fmt"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/pkg/trie"
)

type scripting struct {
	labels, modeAutoComplete *trie.Trie
	rcfiles                  map[string]string
	hostmapping              map[string][]string
}

func (s *scripting) Run(term *terminal.Terminal, args ...string) error {
	if len(args) < 1 {
		helpText := "rc load <label> <rc file path>\n"
		helpText += "rc enable <label> <remote_id>\n"
		helpText += "rc disable <label> <remote_id>\n"
		helpText += "rc ls <remote_id>\n"
		helpText += "rc files"
		return fmt.Errorf(helpText)
	}

	switch args[0] {

	case "load":
		if len(args) != 3 {
			return fmt.Errorf("Not enough args for rc load. rc load <label> <rc file path>")
		}

		if !internal.FileExists(args[2]) {
			return fmt.Errorf("File %s does not exist", args[2])
		}

		s.labels.Add(args[1])
		s.rcfiles[args[1]] = args[2]

		fmt.Fprintf(term, "RC file [%s] loaded to label %s\n", args[2], args[1])

	case "enable", "disable":
		if len(args) != 3 {
			return fmt.Errorf("Not enough args for rc %s. rc %s <label> <remote_id>", args[0], args[0])
		}

		if _, ok := s.rcfiles[args[1]]; !ok {
			return fmt.Errorf("Label not found")
		}

		currentRCFiles, ok := s.hostmapping[args[2]]
		if !ok {
			return fmt.Errorf("Host not found")
		}

		index := -1
		for i := 0; i < len(currentRCFiles); i++ {
			if currentRCFiles[i] == args[1] {
				index = i
				break
			}
		}

		if args[0] == "enable" && index == -1 {
			s.hostmapping[args[2]] = append(currentRCFiles, args[1])
		}

		if args[0] == "disable" && index != -1 {
			currentRCFiles[index] = currentRCFiles[len(currentRCFiles)-1]
			s.hostmapping[args[2]] = currentRCFiles[:len(currentRCFiles)-1]
		}

	case "ls":
		if len(args) != 2 {
			return fmt.Errorf("Not enough args for rc ls. rc ls <remote_id>")
		}

		rcfiles, ok := s.hostmapping[args[1]]
		if !ok {
			return fmt.Errorf("Host [%s] doesnt have any Rc files set", args[1])
		}

		for _, v := range rcfiles {
			fmt.Fprintf(term, "%s\n", v)
		}

	case "files":
		fmt.Fprintf(term, "Files: \n")
		for k, v := range s.rcfiles {
			fmt.Fprintf(term, "%s:[%s]\n", k, v)
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
			return s.labels.PrefixMatch(sections[1])
		}

		if len(sections) == 3 {
			return []string{RemoteId}
		}

	case "ls":
		return []string{RemoteId}

	}

	return nil
}

func RC() *scripting {
	return &scripting{
		rcfiles:          make(map[string]string),
		hostmapping:      make(map[string][]string),
		modeAutoComplete: trie.NewTrie("load", "enable", "disable", "ls", "files"),
		labels:           trie.NewTrie(),
	}
}
