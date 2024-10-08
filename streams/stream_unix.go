//go:build unix

package streams

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

const CurrentStreamImpl = UnixFIFOStreamImpl

func NewStream() (Stream, error) {
	name := filepath.Join(os.TempDir(), getStreamName())
	if err := unix.Mkfifo(name, 0666); err != nil {
		return nil, err
	}
	return &fifoStream{
		name: name,
		done: make(chan struct{}),
	}, nil
}

var ErrNotAFIFO = fmt.Errorf("file is not a fifo")

type fifoStream struct {
	name     string
	blocked  int
	lastFlag int
	done     chan struct{}
	mu       sync.Mutex
	once     sync.Once
}

func (st *fifoStream) isClosed() bool {
	select {
	case <-st.done:
		return true
	default:
		return false
	}
}

func (st *fifoStream) Open(flag int) (DeadlineReadWriteCloser, error) {
	if st.isClosed() {
		return nil, &os.PathError{Op: "open", Path: st.name, Err: os.ErrClosed}
	}

	type result struct {
		f   *os.File
		err error
	}

	resCh := make(chan result)
	go func() {
		st.mu.Lock()
		if st.isClosed() {
			st.mu.Unlock()
			return
		}
		st.blocked++
		st.lastFlag = flag
		st.mu.Unlock()
		// may block indefinitely
		f, err := os.OpenFile(st.name, flag, 0600)
		st.mu.Lock()
		st.blocked--
		st.mu.Unlock()
		select {
		case resCh <- result{f, err}:
		case <-st.done:
			if err == nil {
				f.Close()
			}
		}
	}()
	select {
	case res := <-resCh:
		if res.err != nil {
			return nil, res.err
		}
		info, err := res.f.Stat()
		if err == nil && info.Mode()&os.ModeType != os.ModeNamedPipe {
			err = &os.PathError{Op: "open", Path: st.name, Err: ErrNotAFIFO}
		}
		if err != nil {
			res.f.Close()
			return nil, err
		}
		return res.f, nil
	case <-st.done:
		return nil, &os.PathError{Op: "open", Path: st.name, Err: os.ErrClosed}
	}
}

func (st *fifoStream) Close() error {
	var closeErr error
	st.once.Do(func() {
		st.mu.Lock()
		close(st.done)
		if st.blocked > 0 {
			if err := st.unclog(); err != nil && errors.Is(err, unix.ENOENT) {
				// original FIFO file has been deleted.
				// goroutines that are still blocked on open will leak.
				st.mu.Unlock()
				closeErr = &os.PathError{Op: "close", Path: st.name, Err: err}
				return
			}
		}
		st.mu.Unlock()
		closeErr = os.Remove(st.name)
	})
	return closeErr
}

// unclog tries to unblock goroutines blocked on opening the pipe by opening it
// on the opposite side (read->write, write->read).
func (st *fifoStream) unclog() error {
	var flag int
	switch st.lastFlag & unix.O_ACCMODE {
	case unix.O_RDONLY:
		flag = unix.O_WRONLY
	case unix.O_WRONLY:
		flag = unix.O_RDONLY
	default:
		return nil
	}
	var fd int
	for {
		var err error
		fd, err = unix.Open(st.name, flag|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil {
			// retry if interrupted by a signal
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return err
		}
		break
	}
	unix.Close(fd)
	return nil
}

func (st *fifoStream) Scheme() string {
	return "file"
}

func (st *fifoStream) String() string {
	return st.name
}
