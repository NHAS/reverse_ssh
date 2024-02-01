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
	"github.com/NHAS/reverse_ssh/internal/server/observers"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type autostartEntry struct {
	ObserverID string
	Criteria   string
}

var autoStartServerPort = map[internal.RemoteForwardRequest]autostartEntry{}

type listen struct {
	log logger.Logger
}

func (l *listen) server(tty io.ReadWriter, line terminal.ParsedLine, onAddrs, offAddrs []string) error {
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

	for _, addr := range onAddrs {
		err := multiplexer.ServerMultiplexer.StartListener("tcp", addr)
		if err != nil {
			return err
		}
		fmt.Fprintln(tty, "started listening on: ", addr)
	}

	for _, addr := range offAddrs {
		err := multiplexer.ServerMultiplexer.StopListener(addr)
		if err != nil {
			return err
		}
		fmt.Fprintln(tty, "stopped listening on: ", addr)
	}

	return nil
}

func (l *listen) client(tty io.ReadWriter, line terminal.ParsedLine, onAddrs, offAddrs []string) error {

	auto := line.IsSet("auto")
	if line.IsSet("l") && auto {
		for k, v := range autoStartServerPort {
			fmt.Fprintf(tty, "%s %s:%d\n", v.Criteria, k.BindAddr, k.BindPort)
		}
		return nil
	}

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

	if len(foundClients) == 0 && !auto {
		return fmt.Errorf("No clients matched '%s'", specifier)
	}

	if line.IsSet("l") {

		for id, cc := range foundClients {
			result, message, _ := cc.SendRequest("query-tcpip-forwards", true, nil)
			if !result {
				fmt.Fprintf(tty, "%s does not support querying server forwards\n", id)
				continue
			}

			f := struct {
				RemoteForwards []string
			}{}

			err := ssh.Unmarshal(message, &f)
			if err != nil {
				fmt.Fprintf(tty, "%s sent an incompatiable message: %s\n", id, err)
				continue
			}

			fmt.Fprintf(tty, "%s (%s %s): \n", id, clients.NormaliseHostname(cc.User()), cc.RemoteAddr().String())
			for _, rf := range f.RemoteForwards {
				fmt.Fprintf(tty, "\t%s\n", rf)
			}

		}

		return nil
	}

	var fwRequests []internal.RemoteForwardRequest

	for _, addr := range onAddrs {
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

		applied := len(foundClients)
		for c, sc := range foundClients {
			result, message, err := sc.SendRequest("tcpip-forward", true, b)
			if !result {
				applied--
				fmt.Fprintln(tty, "failed to start port on (client may not support it): ", c, ": ", string(message))
				continue
			}

			if err != nil {
				applied--
				fmt.Fprintln(tty, "error starting port on: ", c, ": ", err)
			}
		}

		fmt.Fprintf(tty, "started %s:%d on %d clients (total %d)\n", r.BindAddr, r.BindPort, applied, len(foundClients))

		if auto {
			var entry autostartEntry

			entry.ObserverID = observers.ConnectionState.Register(func(c observers.ClientState) {

				if !clients.Matches(specifier, c.ID, c.IP) || c.Status == "disconnected" {
					return
				}

				client, err := clients.Get(c.ID)
				if err != nil {
					return
				}

				result, message, err := client.SendRequest("tcpip-forward", true, b)
				if !result {
					l.log.Warning("failed to start server tcpip-forward on client: %s: %s", c.ID, message)
					return
				}

				if err != nil {
					l.log.Warning("error auto starting port on: %s: %s", c.ID, err)
					return
				}

			})

			entry.Criteria = specifier

			autoStartServerPort[r] = entry

		}
	}

	var cancelFwRequests []internal.RemoteForwardRequest

	for _, addr := range offAddrs {
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
		applied := len(foundClients)

		b := ssh.Marshal(&r)
		for c, sc := range foundClients {
			result, message, err := sc.SendRequest("cancel-tcpip-forward", true, b)
			if !result {
				applied--
				fmt.Fprintln(tty, "failed to stop port on: ", c, ": ", string(message))
				continue
			}

			if err != nil {
				applied--
				fmt.Fprintln(tty, "error stop port on: ", c, ": ", err)
			}
		}

		fmt.Fprintf(tty, "stopped %s:%d on %d clients\n", r.BindAddr, r.BindPort, applied)

		if auto {
			if _, ok := autoStartServerPort[r]; ok {
				observers.ConnectionState.Deregister(autoStartServerPort[r].Criteria)
			}
			delete(autoStartServerPort, r)
		}
	}

	return nil
}

func (w *listen) Run(tty io.ReadWriter, line terminal.ParsedLine) error {
	if line.IsSet("h") || len(line.Flags) < 1 {
		fmt.Fprintf(tty, "%s", w.Help(false))
		return nil
	}

	onAddrs, err := line.GetArgsString("on")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	if len(onAddrs) == 0 && err != terminal.ErrFlagNotSet {
		return errors.New("no value specified for --on, requires port e.g --on :4343")
	}

	offAddrs, err := line.GetArgsString("off")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	if len(offAddrs) == 0 && err != terminal.ErrFlagNotSet {
		return errors.New("no value specified for --off, requires port e.g --off :4343")
	}

	if onAddrs == nil && offAddrs == nil && !line.IsSet("l") {
		return errors.New("no actionable argument supplied, please add --on, --off or -l (list)")
	}

	if line.IsSet("server") || line.IsSet("s") {
		return w.server(tty, line, onAddrs, offAddrs)
	} else if line.IsSet("client") || line.IsSet("c") || line.IsSet("auto") {
		return w.client(tty, line, onAddrs, offAddrs)
	}

	return errors.New("neither server or client were specified, please choose one")
}

func (W *listen) Expect(line terminal.ParsedLine) []string {

	if line.Section != nil {
		switch line.Section.Value() {
		case "c", "client":
			return []string{autocomplete.RemoteId}
		}
	}

	return nil
}

func (w *listen) Help(explain bool) string {
	if explain {
		return "Change, add or stop rssh server port. Open the server port on a client (proxy)"
	}

	return terminal.MakeHelpText(
		"listen [OPTION] [PORT]",
		"listen starts or stops listening control ports",
		"it allows you to change the servers listening port, or open the servers control port on an rssh client, so that forwarding is easier",
		"\t--client (-c)\tOpen server port on client/s takes a pattern, e.g -c *, --client your.hostname.here",
		"\t--server (-s)\tChange the server listeners",
		"\t--on\tTurn on port, e.g --on :8080 127.0.0.1:4444",
		"\t--auto\tAutomatically turn on server control port on clients that match criteria, (use --off --auto to disable and --l --auto to view)",
		"\t--off\tTurn off port, e.g --off :8080 127.0.0.1:4444",
		"\t-l\tList all enabled addresses",
	)
}

func Listen(log logger.Logger) *listen {
	return &listen{
		log: log,
	}
}
