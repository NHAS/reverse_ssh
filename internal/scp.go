package internal

import "io"

type Scp struct {
	Mode string
	Path string
}

func ScpError(severity int, reason string, connection io.Writer) {
	connection.Write([]byte{byte(severity)})
	connection.Write([]byte(reason + "\n"))
}
