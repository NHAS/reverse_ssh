package commands

import (
	"strings"
)

func syntaxParse(syntax string, section int) string {

	parts := strings.Split(syntax, " ")

	if len(parts)-1 < section {
		return ""
	}

	return parts[section]

}
