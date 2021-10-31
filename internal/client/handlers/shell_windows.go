//go:build windows
// +build windows

package handlers

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/ActiveState/termtest/conpty"
	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/client/term"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/windows"
)

//The basic windows shell handler, as there arent any good golang libraries to work with windows conpty
func shell(user *internal.User, connection ssh.Channel, requests <-chan *ssh.Request, log logger.Logger) {

	if user.Pty == nil {
		fmt.Fprintln(connection, "You need a pty to be able to use terminal")
		return
	}

	vsn := windows.RtlGetVersion()
	if vsn.MajorVersion < 10 || vsn.BuildNumber < 17763 {
		log.Info("Windows version too old for Conpty (%d, %d), using basic shell", vsn.MajorVersion, vsn.BuildNumber)
		if shellhostShell(connection, requests) != nil {
			basicShell(connection, requests, log)
		}
	} else {
		err := conptyShell(connection, requests, log, *user.Pty)
		if err != nil {
			log.Error("%v", err)
		}
	}

	connection.Close()

}

func conptyShell(connection ssh.Channel, reqs <-chan *ssh.Request, log logger.Logger, ptyReq internal.PtyReq) error {

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

func shellhostShell(connection ssh.Channel, reqs <-chan *ssh.Request) error {

	end := make(chan bool, 1)
	go func() {
		for {
			select {
			case <-end:
				return
			case r := <-reqs:
				if r.WantReply {
					r.Reply(false, nil)
				}
			}
		}
	}()
	d, _ := os.Executable()
	cmd := exec.Command(d, "--exec", "powershell.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	stdout, _ := cmd.StdoutPipe()
	stdin, _ := cmd.StdinPipe()

	cmd.Stderr = cmd.Stdout

	err := cmd.Start()
	if err != nil {
		return err

	}
	defer cmd.Process.Kill()

	go io.Copy(stdin, connection)
	io.Copy(connection, stdout)

	if cmd.ProcessState.ExitCode() == 1 {
		return errors.New("Shell host client failed with error")
	}

	return nil
}

func basicShell(connection ssh.Channel, reqs <-chan *ssh.Request, log logger.Logger) {

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

	terminal := term.NewTerminal(connection, "")
	// Dynamically handle resizes of terminal window
	go func() {
		for req := range reqs {
			switch req.Type {

			case "window-change":
				w, h := internal.ParseDims(req.Payload)
				terminal.SetSize(int(w), int(h))

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
			_, err = terminal.Write(buf[ignoredByes:n])
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
			line, err := terminal.ReadLine()
			if err != nil && err != term.ErrCtrlC {
				log.Error("%s", err)
				return
			}

			if err == term.ErrCtrlC {
				expected <- true
				err := sendCtrlC(cmd.Process.Pid)
				if err != nil {
					fmt.Fprintf(terminal, "Failed to send Ctrl+C sorry! You are most likely trapped: %s", err)
					log.Error("%s", err)
				}
			}

			if err == nil {
				_, err := stdin.Write([]byte(line + "\r\n"))
				if err != nil {
					fmt.Fprintf(terminal, "Error writing to STDIN: %s", err)
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
