package tcp

import (
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/NHAS/reverse_ssh/internal/server/data"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

func handleBashConn(conn net.Conn) {
	defer conn.Close()

	downloadLog := logger.NewLog(conn.RemoteAddr().String())

	conn.SetDeadline(time.Now().Add(3 * time.Second))
	// RAW header prefix + 64 bytes for file ID
	fileID := make([]byte, 67)

	n, err := conn.Read(fileID)
	if err != nil {
		downloadLog.Warning("failed to download file using raw tcp: %s", err)
		return
	}

	conn.SetDeadline(time.Time{})

	if n == 0 || n < 3 {
		downloadLog.Warning("recieved malformed raw download request")
		return
	}

	filename := strings.TrimSpace(string(fileID[3:n]))

	f, err := data.GetDownload(filename)
	if err != nil {
		downloadLog.Warning("failed to get file %q: err %s", filename, err)
		return
	}

	file, err := os.Open(f.FilePath)
	if err != nil {
		downloadLog.Warning("failed to open file %q for download: %s", f.FilePath, err)
		return
	}
	defer file.Close()

	downloadLog.Info("downloaded %q using RAW tcp method", filename)

	io.Copy(conn, file)
}

func Start(listener net.Listener) {

	log.Println("Started Raw Download Server")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept raw download connection: %s", err)
			return
		}

		go handleBashConn(conn)
	}
}
