package commands

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/clients"
	"github.com/NHAS/reverse_ssh/internal/server/multiplexer"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type listen struct {
	log logger.Logger
}

func server(tty io.ReadWriter, line terminal.ParsedLine) error {
	if line.IsSet("l") {
		listeners := multiplexer.ServerMultiplexer.GetListeners()

		if len(listeners) == 0 {
			fmt.Fprintln(tty, "No active listeners")
			return nil
		}

		for _, listener := range listeners {
			fmt.Fprintf(tty, "%s\n", listener)
		}
		return nil
	}

	on := line.IsSet("on")
	off := line.IsSet("off")

	if on {
		addrs, err := line.GetArgsString("on")
		if err != nil {
			return err
		}

		for _, addr := range addrs {
			err := multiplexer.ServerMultiplexer.StartListener("tcp", addr)
			if err != nil {
				return err
			}
			fmt.Fprintln(tty, "started listening on: ", addr)
		}
	}

	if off {
		addrs, err := line.GetArgsString("off")
		if err != nil {
			return err
		}

		for _, addr := range addrs {
			err := multiplexer.ServerMultiplexer.StopListener(addr)
			if err != nil {
				return err
			}
			fmt.Fprintln(tty, "stopped listening on: ", addr)
		}
	}

	return nil
}

func client(tty io.ReadWriter, line terminal.ParsedLine) error {

	specifier, err := line.GetArgString("c")
	if err != nil {
		specifier, err = line.GetArgString("client")
		if err != nil {
			return err
		}
	}

	foundClients, err := clients.Search(specifier)
	if err != nil {
		return err
	}

	if len(foundClients) == 0 {
		return fmt.Errorf("No clients matched '%s'", client)
	}

	on := line.IsSet("on")
	off := line.IsSet("off")

	if on {
		var fwRequests []internal.RemoteForwardRequest

		addrs, err := line.GetArgsString("on")
		if err != nil {
			return err
		}

		for _, addr := range addrs {
			ip, port, err := net.SplitHostPort(addr)
			if err != nil {
				return err
			}

			p, err := strconv.Atoi(port)
			if err != nil {
				return err
			}

			fwRequests = append(fwRequests, internal.RemoteForwardRequest{
				BindPort: uint32(p),
				BindAddr: ip,
			})

		}

		for _, r := range fwRequests {
			b := ssh.Marshal(&r)
			for c, sc := range foundClients {
				result, message, err := sc.SendRequest("tcpip-forward", true, b)
				if !result {
					fmt.Fprintln(tty, "failed to start port on: ", c, ": ", message)
					continue
				}

				if err != nil {
					fmt.Fprintln(tty, "error starting port on: ", c, ": ", err)
				}
			}
		}

	}

	if off {
		var cancelFwRequests []internal.RemoteForwardRequest

		addrs, err := line.GetArgsString("off")
		if err != nil {
			return err
		}

		for _, addr := range addrs {
			ip, port, err := net.SplitHostPort(addr)
			if err != nil {
				return err
			}

			p, err := strconv.Atoi(port)
			if err != nil {
				return err
			}

			cancelFwRequests = append(cancelFwRequests, internal.RemoteForwardRequest{
				BindPort: uint32(p),
				BindAddr: ip,
			})

		}

		for _, r := range cancelFwRequests {
			b := ssh.Marshal(&r)
			for c, sc := range foundClients {
				result, message, err := sc.SendRequest("cancel-tcpip-forward", true, b)
				if !result {
					fmt.Fprintln(tty, "failed to stop port on: ", c, ": ", message)
					continue
				}

				if err != nil {
					fmt.Fprintln(tty, "error stop port on: ", c, ": ", err)
				}
			}
		}
	}

	return nil
}

func (w *listen) Run(tty io.ReadWriter, line terminal.ParsedLine) error {
	if line.IsSet("h") || len(line.Flags) < 1 {
		fmt.Fprintf(tty, "%s", w.Help(false))
		return nil
	}

	if line.IsSet("server") || line.IsSet("s") {
		return server(tty, line)
	} else if line.IsSet("client") || line.IsSet("c") {
		return client(tty, line)
	}

	return errors.New("neither server or client were specified, please choose one")
}

func (W *listen) Expect(line terminal.ParsedLine) []string {
	return nil
}

func (w *listen) Help(explain bool) string {
	if explain {
		return "listen changes the rssh server ports, start or stop multiple listening ports"
	}

	return terminal.MakeHelpText(
		"listen [OPTION] [PORT]",
		"listen starts or stops listening control ports",
		"\t--client (-c)\tSpecify client/s to act on, e.g -c *, --client your.hostname.here",
		"\t--server (-s)\tSpecify to change the server listeners",
		"\t--on\tTurn on port, e.g --on :8080 127.0.0.1:4444",
		"\t--off\tTurn off port, e.g --off :8080 127.0.0.1:4444",
		"\t-l\tList all enabled addresses",
	)
}

func Listen(log logger.Logger) *listen {
	return &listen{
		log: log,
	}
}
