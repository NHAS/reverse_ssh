package mux

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net"
	"runtime/debug"
	"time"
)

var ErrClosed = errors.New("fragment collector has been closed")

type fragmentedConnection struct {
	done chan interface{}

	readBuffer  *SyncBuffer
	writeBuffer *SyncBuffer
}

func NewFragmentCollector() (*fragmentedConnection, string, error) {

	fc := &fragmentedConnection{
		done: make(chan interface{}),

		readBuffer:  NewSyncBuffer(),
		writeBuffer: NewSyncBuffer(),
	}

	randomData := make([]byte, 16)
	_, err := rand.Read(randomData)
	if err != nil {
		return nil, "", err
	}

	id := hex.EncodeToString(randomData)

	return fc, id, nil
}

func (fc *fragmentedConnection) Read(b []byte) (n int, err error) {

	select {
	case <-fc.done:
		return 0, io.EOF
	default:

	}

	return fc.readBuffer.BlockingRead(b)
}

func (fc *fragmentedConnection) Write(b []byte) (n int, err error) {

	select {
	case <-fc.done:
		return 0, io.EOF
	default:

	}
	return fc.readBuffer.BlockingWrite(b)
}

func (fc *fragmentedConnection) Close() error {

	log.Println("closed? ", string(debug.Stack()))
	fc.writeBuffer.Close()
	fc.readBuffer.Close()

	select {
	case <-fc.done:
	default:

		close(fc.done)
	}

	return nil
}

func (fc *fragmentedConnection) LocalAddr() net.Addr {
	return &net.TCPAddr{}
}

func (fc *fragmentedConnection) RemoteAddr() net.Addr {
	return &net.TCPAddr{}
}

func (fc *fragmentedConnection) SetDeadline(t time.Time) error {
	return nil
}

func (fc *fragmentedConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (fc *fragmentedConnection) SetWriteDeadline(t time.Time) error {
	return nil
}
