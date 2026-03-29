package vfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

// MultipartFile is an interface that extends io.ReadSeekCloser with ReadAt method
type MultipartFile interface {
	io.ReadSeekCloser
	ReadAt(p []byte, off int64) (n int, err error)
}

type ReadSeekCloser struct {
	io.Seeker
	io.Closer

	br *bytes.Reader
	mu *sync.RWMutex
}

// CopyWithContext performs io.Copy with context cancellation checks
func CopyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (written int64, err error) {
	buf := make([]byte, 32*1024) // 32KB chunks
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}

		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

// NewReadSeekCloser implements a bytes.Reader and emulates io.Closer and io.Seeker
func NewReadSeekCloser(b []byte) MultipartFile {
	mu := new(sync.RWMutex)
	rsc := &ReadSeekCloser{
		mu: mu,
		br: bytes.NewReader(b),
	}
	return rsc
}

// Read implements the standard Read interface
func (rsc *ReadSeekCloser) Read(p []byte) (n int, err error) {
	return rsc.br.Read(p)
}

// ReadAt implements the ReadAt method from MultipartFile
func (rsc *ReadSeekCloser) ReadAt(p []byte, off int64) (n int, err error) {
	rsc.mu.RLock()
	defer rsc.mu.RUnlock()
	return rsc.br.ReadAt(p, off)
}

// Close closes the reader
func (rsc *ReadSeekCloser) Close() error {
	rsc.mu.Lock()
	defer rsc.mu.Unlock()
	rsc.br = nil
	return nil
}

// Seek implements the `io.Seeker` interface.
func (rsc *ReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	return rsc.br.Seek(offset, whence)
}
