package commands

func makeHelpText(lines ...string) (s string) {
	for _, v := range lines {
		s += v + "\n"
	}

	return s
}

func parseFlags(args ...string) (flags map[string][]string, leftover []string) {
	flags = map[string][]string{}
	capture := ""
	for _, arg := range args {
		if len(arg) > 1 && arg[0] == '-' {
			//Enable long option parsing --blah
			if arg[1] == '-' {
				if len(arg) == 2 {
					//Ignore "--"
					continue
				}

				flags[arg[2:]] = nil
				capture = arg[2:]
				continue
			}

			//Start short option parsing -l or -ltab = -l -t -a -b
			for _, c := range arg[1:] {
				flags[string(c)] = nil
			}

			//Most of the time its ambigous with multiple options what arg goes with what option
			capture = ""

			//For a single option, its not ambigous for what option we're capturing an arg for
			if len(arg) == 2 {
				capture = string(arg[1])
			}
			continue
		}

		if len(capture) > 0 {
			flags[capture] = append(flags[capture], arg)
			continue
		}

		leftover = append(leftover, arg)
	}

	return

}

func isSet(flag string, flagmap map[string][]string) bool {
	_, ok := flagmap[flag]
	return ok
}
