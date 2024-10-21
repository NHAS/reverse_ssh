package commands

import (
	"errors"
	"fmt"
	"io"

	"github.com/NHAS/reverse_ssh/internal/server/data"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
)

type webhook struct {
}

func (w *webhook) ValidArgs() map[string]string {
	return map[string]string{
		"on":       "Turns on webhook/s, must supply output as url",
		"off":      "Turns off existing webhook url",
		"insecure": "Disable TLS certificate checking",
		"l":        "Lists active webhooks",
	}
}

func (w *webhook) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {
	if len(line.Flags) < 1 {
		fmt.Fprintf(tty, "%s", w.Help(false))
		return nil
	}

	if line.IsSet("l") {
		webhooks, err := data.GetAllWebhooks()
		if err != nil {
			return err
		}

		if len(webhooks) == 0 {
			fmt.Fprintln(tty, "No active listeners")
			return nil
		}

		for _, listener := range webhooks {
			fmt.Fprintf(tty, "%s\n", listener.URL)
		}
		return nil
	}

	on := line.IsSet("on")
	off := line.IsSet("off")

	if on && off {
		return errors.New("cannot specify on and off at the same time")
	}

	if on {

		addrs, err := line.GetArgsString("on")
		if err != nil {
			return err
		}

		for i, addr := range addrs {
			resultingUrl, err := data.CreateWebhook(addr, !line.IsSet("insecure"))
			if err != nil {
				fmt.Fprintf(tty, "(%d/%d) Failed: %s, reason: %s\n", i+1, len(addrs), resultingUrl, err.Error())
				continue
			}

			fmt.Fprintf(tty, "(%d/%d) Enabled webhook: %s\n", i+1, len(addrs), resultingUrl)
		}

		return nil

	}

	if off {
		existingWebhooks, err := line.GetArgsString("off")
		if err != nil {
			return err
		}

		for i, hook := range existingWebhooks {
			err := data.DeleteWebhook(hook)
			if err != nil {
				fmt.Fprintf(tty, "(%d/%d) Failed to remove: %s, reason: %s\n", i+1, len(existingWebhooks), hook, err.Error())
				continue
			}

			fmt.Fprintf(tty, "(%d/%d) Disabled webhook: %s\n", i+1, len(existingWebhooks), hook)
		}
		return nil

	}

	return nil

}

func (w *webhook) Expect(line terminal.ParsedLine) []string {
	return nil
}

func (w *webhook) Help(explain bool) string {
	if explain {
		return "Add or remove webhooks"
	}

	return terminal.MakeHelpText(w.ValidArgs(),
		"webhook [OPTIONS]",
		"Allows you to set webhooks which currently show the joining and leaving of clients",
	)
}
