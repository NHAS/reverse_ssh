package observers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/NHAS/reverse_ssh/pkg/observer"
	"github.com/fatih/color"
)

type ClientState struct {
	Status    string
	ID        string
	IP        string
	HostName  string
	Version   string
	Timestamp time.Time
}

func (wd ClientState) String() string {

	var (
		arrowDirection = "<-"
		statusColor    = color.GreenString
	)
	if wd.Status == "disconnected" {
		statusColor = color.RedString
		arrowDirection = "->"
	}

	return fmt.Sprintf("%s %s %s (%s %s) %s %s",
		wd.Timestamp.Format("2006/01/02 15:04:05"),
		arrowDirection,
		color.BlueString(wd.HostName),
		wd.IP,
		color.YellowString(wd.ID),
		wd.Version, statusColor(wd.Status))
}

func (cs ClientState) Summary() string {
	return fmt.Sprintf("%s (%s) %s %s", cs.HostName, cs.ID, cs.Version, cs.Status)
}

func (cs ClientState) Json() ([]byte, error) {
	return json.Marshal(cs)
}

var ConnectionState = observer.New[ClientState]()
