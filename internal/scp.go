package internal

import "io"

type Scp struct {
	Mode string
	Path string
}

func ScpError(reason string, connection io.Writer) {
	connection.Write([]byte{2})
	connection.Write([]byte(reason + "\n"))
	connection.Write([]byte("E\n"))
}
