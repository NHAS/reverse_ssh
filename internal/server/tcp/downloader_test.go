package tcp

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestReadRawDownloadName(t *testing.T) {
	tests := []struct {
		name    string
		chunks  []string
		want    string
		wantErr bool
	}{
		{
			name:   "shell script",
			chunks: []string{"RAWclient.sh\n"},
			want:   "client.sh",
		},
		{
			name:   "powershell script",
			chunks: []string{"RAWclient.ps1\r\n"},
			want:   "client.ps1",
		},
		{
			name:   "raw binary without newline",
			chunks: []string{"RAWpayload.bin"},
			want:   "payload.bin",
		},
		{
			name:   "max length filename",
			chunks: []string{"RAW" + strings.Repeat("a", rawDownloadMaxNameLength) + "\n"},
			want:   strings.Repeat("a", rawDownloadMaxNameLength),
		},
		{
			name:   "split raw request",
			chunks: []string{"R", "AW", "payload.bin", "\n"},
			want:   "payload.bin",
		},
		{
			name:    "missing raw prefix",
			chunks:  []string{"GET /payload.bin HTTP/1.1\r\n"},
			wantErr: true,
		},
		{
			name:    "empty filename",
			chunks:  []string{"RAW\n"},
			wantErr: true,
		},
		{
			name:    "filename too long",
			chunks:  []string{"RAW" + strings.Repeat("a", rawDownloadMaxNameLength+1) + "\n"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &chunkedConn{chunks: byteChunks(tt.chunks)}

			got, err := readRawDownloadName(conn)
			if !conn.readDeadline.IsZero() {
				t.Fatal("read deadline was not reset")
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("readRawDownloadName failed: %v", err)
			}
			if got != tt.want {
				t.Fatalf("filename = %q, want %q", got, tt.want)
			}
		})
	}
}

func byteChunks(chunks []string) [][]byte {
	result := make([][]byte, 0, len(chunks))
	for _, chunk := range chunks {
		result = append(result, []byte(chunk))
	}
	return result
}

type chunkedConn struct {
	chunks       [][]byte
	readDeadline time.Time
}

func (c *chunkedConn) Read(b []byte) (int, error) {
	if len(c.chunks) == 0 {
		return 0, io.EOF
	}

	n := copy(b, c.chunks[0])
	if n == len(c.chunks[0]) {
		c.chunks = c.chunks[1:]
	} else {
		c.chunks[0] = c.chunks[0][n:]
	}
	return n, nil
}

func (c *chunkedConn) Write([]byte) (int, error) {
	return 0, nil
}

func (c *chunkedConn) Close() error {
	return nil
}

func (c *chunkedConn) LocalAddr() net.Addr {
	return nil
}

func (c *chunkedConn) RemoteAddr() net.Addr {
	return nil
}

func (c *chunkedConn) SetDeadline(time.Time) error {
	return nil
}

func (c *chunkedConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

func (c *chunkedConn) SetWriteDeadline(time.Time) error {
	return nil
}
