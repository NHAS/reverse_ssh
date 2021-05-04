package commands

import (
	"fmt"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/pkg/trie"
)

type scripting struct {
	modeAutoComplete *trie.Trie
}

func (s *scripting) Run(term *terminal.Terminal, args ...string) error {
	if len(args) != 1 {
		helpText := "rc load <label> <rc file path>\n"
		helpText += "rc enable <label> <host>\n"
		helpText += "rc disable <label> <host>"
		return fmt.Errorf(helpText)
	}

	switch args[0] {
	case "load":
		if len(args) != 3 {
			return fmt.Errorf("Not enough args for rc load. rc load <label> <rc file path>")
		}

	case "enable":
	case "disable":
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
	case "load":

	case "enable":
	case "disable":
	default:
		return nil
	}

	return nil
}

func RC() *scripting {
	mt := trie.NewTrie("load", "enable", "disable")

	return &scripting{
		modeAutoComplete: mt,
	}
}
