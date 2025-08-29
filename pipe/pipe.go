// WARNING: DO NOT USE THIS.  Use os.Pipe instead.
//
// The implementation is included for (chapter 4) completeness.
package pipe

import (
	"fmt"
	"io"
	"syscall"
)

type file struct {
	fd int

	// NOTE: since file descriptor numbers are reused by the process, we'll
	// need to ensure no operation can occur after close.
	closed   bool
	closeErr error
}

func (f *file) Read(p []byte) (int, error) {
	if f.closed {
		return 0, fmt.Errorf("invalid operation. reading from a closed file.")
	}

	n, err := syscall.Read(f.fd, p)
	if err != nil {
		return n, fmt.Errorf("failed to read from fd %d: %w", f.fd, err)
	}

	return n, nil
}

func (f *file) Write(p []byte) (int, error) {
	if f.closed {
		return 0, fmt.Errorf("invalid operation. writing to a closed file.")
	}

	n, err := syscall.Write(f.fd, p)
	if err != nil {
		return n, fmt.Errorf("failed to write to fd %d: %w", f.fd, err)
	}

	return n, nil
}

func (f *file) Close() error {
	if f.closed {
		return f.closeErr
	}

	err := syscall.Close(f.fd)
	if err != nil {
		err = fmt.Errorf("failed to close fd %d: %w", f.fd, err)
	}

	f.closed = true
	f.closeErr = err

	return err
}

func Pipe() (io.ReadCloser, io.WriteCloser, error) {
	fds := make([]int, 2)

	// NOTE: Unlike the book, we'll always close on exec.
	err := syscall.Pipe2(fds, syscall.O_CLOEXEC)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pipe: %w", err)
	}

	reader := &file{
		fd: fds[0],
	}

	writer := &file{
		fd: fds[1],
	}

	return reader, writer, nil
}
