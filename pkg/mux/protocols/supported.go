package protocols

type Type string

const (

	// Wrappers/Transports
	Websockets Type = "ws"
	HTTP       Type = "polling"
	TLS        Type = "tls"

	// Final control/data channel
	Download Type = "download"
	C2       Type = "ssh"

	Invalid Type = "invalid"
)

func FullyUnwrapped(currentProtocol Type) bool {
	return currentProtocol == C2 || currentProtocol == Download
}
