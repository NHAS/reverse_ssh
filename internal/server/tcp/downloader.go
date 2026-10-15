package tcp

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/NHAS/reverse_ssh/internal/server/data"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

const (
	rawDownloadPrefix        = "RAW"
	rawDownloadMaxNameLength = 64
	rawDownloadReadTimeout   = 3 * time.Second
)

func handleBashConn(conn net.Conn) {
	defer conn.Close()

	downloadLog := logger.NewLog(conn.RemoteAddr().String())

	filename, err := readRawDownloadName(conn)
	if err != nil {
		downloadLog.Warning("failed to download file using raw tcp: %s", err)
		return
	}

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

func readRawDownloadName(conn net.Conn) (string, error) {
	_ = conn.SetReadDeadline(time.Now().Add(rawDownloadReadTimeout))
	defer conn.SetReadDeadline(time.Time{})

	limit := len(rawDownloadPrefix) + rawDownloadMaxNameLength + 1
	request := make([]byte, 0, limit)
	var b [1]byte

	for len(request) < limit {
		n, err := conn.Read(b[:])
		if n > 0 {
			if b[0] == '\n' {
				break
			}
			request = append(request, b[0])
		}

		if err != nil {
			if errors.Is(err, io.EOF) && len(request) > 0 {
				break
			}
			return "", err
		}
	}

	if len(request) >= limit {
		return "", fmt.Errorf("raw download request exceeds %d bytes", limit)
	}

	requestString := strings.TrimSpace(string(request))
	if !strings.HasPrefix(requestString, rawDownloadPrefix) {
		return "", fmt.Errorf("malformed raw download request")
	}

	filename := strings.TrimSpace(strings.TrimPrefix(requestString, rawDownloadPrefix))
	if filename == "" {
		return "", fmt.Errorf("empty raw download filename")
	}
	if len(filename) > rawDownloadMaxNameLength {
		return "", fmt.Errorf("raw download filename exceeds %d bytes", rawDownloadMaxNameLength)
	}

	return filename, nil
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
