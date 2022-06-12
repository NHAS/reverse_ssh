package observers

import (
	"time"

	"github.com/NHAS/reverse_ssh/pkg/observer"
)

type ClientState struct {
	Status    string
	ID        string
	IP        string
	HostName  string
	Timestamp time.Time
}

var ConnectionState = observer.New(ClientState{})
