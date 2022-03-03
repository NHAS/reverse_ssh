package commands

func makeHelpText(lines ...string) (s string) {
	for _, v := range lines {
		s += v + "\n"
	}

	return s
}
