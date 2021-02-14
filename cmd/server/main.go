package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/server"
)

func printHelp() {
	fmt.Println("Reverse SSH server")
	fmt.Println(os.Args[0], "listen addr")
}

func main() {
	arg := ""

	if len(os.Args) > 1 {
		arg = strings.TrimSpace(os.Args[1])
	}

	if len(os.Args) != 2 {
		fmt.Println("Missing listening address")
		printHelp()
		return
	}

	switch arg {
	case "-h", "-help", "--help":
		printHelp()
	default:
		server.Run(os.Args[1])

	}

}
