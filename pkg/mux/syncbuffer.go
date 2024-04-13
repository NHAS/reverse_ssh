package mux

import (
	"bytes"
	"encoding/binary"
	"io"
	"sync"
)

type SyncBuffer struct {
	bb *bytes.Buffer
	sync.Mutex

	rwait sync.Cond
	wwait sync.Cond

	maxLength int

	currentChunk []byte

	isClosed bool
}

// Read from the internal buffer, wait if the buffer is EOF until it is has something to return
func (sb *SyncBuffer) BlockingRead(p []byte) (n int, err error) {
	sb.Lock()
	defer sb.wwait.Signal()
	defer sb.Unlock()

	if sb.isClosed {
		return 0, ErrClosed
	}

	n, err = sb.doRead(p)
	if err == io.EOF {
		for err == io.EOF {

			sb.wwait.Signal()
			sb.rwait.Wait()

			if sb.isClosed {
				return 0, ErrClosed
			}

			n, err = sb.doRead(p)
		}
		return
	}

	return
}

// Read contents of internal buffer, non-blocking and can return eof even if the buffer is still "open"
func (sb *SyncBuffer) Read(p []byte) (n int, err error) {

	sb.Lock()
	defer sb.wwait.Signal()
	defer sb.Unlock()

	return sb.doRead(p)
}

func (sb *SyncBuffer) doRead(p []byte) (n int, err error) {

	if sb.isClosed {
		return 0, ErrClosed
	}

	if sb.currentChunk == nil || len(sb.currentChunk) == 0 {
		headerUint32 := make([]byte, 4)
		n, err = sb.bb.Read(headerUint32)
		if err != nil {
			return 0, err
		}

		if n != 4 {
			panic("header was of incorrect length")
		}

		chunkLength := binary.LittleEndian.Uint32(headerUint32)

		sb.currentChunk = make([]byte, chunkLength)

		sb.bb.Read(sb.currentChunk)
	}

	n = copy(p, sb.currentChunk[:min(len(p), len(sb.currentChunk))])
	sb.currentChunk = sb.currentChunk[n:]

	return n, nil
}

// Write to the internal buffer, but if the buffer is too full block until the pressure has been relieved
func (sb *SyncBuffer) BlockingWrite(p []byte) (n int, err error) {
	sb.Lock()
	defer sb.rwait.Signal()
	defer sb.Unlock()

	if sb.isClosed {
		return 0, ErrClosed
	}

	// In instances that blocking write is being used, Write() is not, its implicit and bad but we assume the starting buffer is 0
	n, err = sb.doWrite(p)
	if err != nil {
		return 0, err
	}
	for {

		sb.rwait.Signal()
		sb.wwait.Wait()

		if sb.isClosed {
			return 0, ErrClosed
		}

		if sb.bb.Len() == 0 {
			return len(p), nil
		}
	}
}

// Write to the internal in-memory buffer, will not block
// This can return ErrClosed if the buffer was closed
func (sb *SyncBuffer) Write(p []byte) (n int, err error) {
	sb.Lock()
	defer sb.rwait.Signal()
	defer sb.Unlock()

	return sb.doWrite(p)
}

func (sb *SyncBuffer) doWrite(p []byte) (n int, err error) {
	if sb.isClosed {
		return 0, ErrClosed
	}

	chunkLength := make([]byte, 4)

	binary.LittleEndian.PutUint32(chunkLength, uint32(len(p)))

	_, err = sb.bb.Write(append(chunkLength, p...))
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

// Threadsafe len()
func (sb *SyncBuffer) Len() int {

	sb.Lock()
	defer sb.Unlock()

	return sb.bb.Len()
}

func (sb *SyncBuffer) Reset() {

	sb.Lock()
	defer sb.Unlock()

	sb.bb.Reset()
}

// Close, resets the internal buffer, wakes all blocking reads/writes
// Double close is a no-op
func (sb *SyncBuffer) Close() error {
	sb.Lock()
	defer sb.Unlock()

	if sb.isClosed {
		return nil
	}

	sb.isClosed = true

	sb.rwait.Signal()
	sb.wwait.Signal()

	sb.bb.Reset()

	return nil
}

func NewSyncBuffer(maxLength int) *SyncBuffer {

	sb := &SyncBuffer{
		bb:        bytes.NewBuffer(nil),
		isClosed:  false,
		maxLength: maxLength,
	}

	sb.rwait.L = &sb.Mutex
	sb.wwait.L = &sb.Mutex

	return sb

}
