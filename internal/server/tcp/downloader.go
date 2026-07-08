package tcp

import (
	"bufio"
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

	reader := bufio.NewReaderSize(conn, len(rawDownloadPrefix)+rawDownloadMaxNameLength+1)
	request, err := readRawDownloadRequest(reader)
	if err != nil {
		return "", err
	}

	request = strings.TrimSpace(request)
	if !strings.HasPrefix(request, rawDownloadPrefix) {
		return "", fmt.Errorf("malformed raw download request")
	}

	filename := strings.TrimSpace(strings.TrimPrefix(request, rawDownloadPrefix))
	if filename == "" {
		return "", fmt.Errorf("empty raw download filename")
	}
	if len(filename) > rawDownloadMaxNameLength {
		return "", fmt.Errorf("raw download filename exceeds %d bytes", rawDownloadMaxNameLength)
	}

	return filename, nil
}

func readRawDownloadRequest(reader *bufio.Reader) (string, error) {
	var request []byte
	limit := len(rawDownloadPrefix) + rawDownloadMaxNameLength + 1

	for {
		fragment, err := reader.ReadSlice('\n')
		request = append(request, fragment...)
		if len(request) > limit {
			return "", fmt.Errorf("raw download request exceeds %d bytes", limit)
		}

		switch {
		case err == nil:
			return string(request), nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(request) > 0:
			return string(request), nil
		default:
			return "", err
		}
	}
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
