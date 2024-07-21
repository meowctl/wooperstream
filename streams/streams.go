package streams

import (
	"fmt"
	"io"
	"math/bits"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// Current Stream implementation.
type StreamImpl int

const (
	TCPSocketStreamImpl StreamImpl = iota
	UnixFIFOStreamImpl
	Win32PipeStreamImpl
)

func (st StreamImpl) String() string {
	switch st {
	case TCPSocketStreamImpl:
		return "TCP socket"
	case UnixFIFOStreamImpl:
		return "Unix named pipe"
	case Win32PipeStreamImpl:
		return "Windows named pipe"
	default:
		return fmt.Sprintf("<%d>", st)
	}
}

type Deadliner interface {
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

type DeadlineReadWriteCloser interface {
	Deadliner
	io.ReadWriteCloser
}

type Stream interface {
	// Open waits for and returns a reader/writer according to the access mode
	// specified.
	Open(flag int) (DeadlineReadWriteCloser, error)

	// Close closes the stream, making any blocked Open calls return errors.
	Close() error

	Scheme() string // name of the stream's scheme, e.g. "tcp".
	String() string // stream's path/network address in string form
}

// FromListener returns a Stream that calls a [net.Listener] underneath.
// Streams created in this way will ignore file access mode flags.
func FromListener(ln net.Listener, scheme string) (Stream, error) {
	return &listenerStream{ln, scheme}, nil
}

type listenerStream struct {
	ln     net.Listener
	scheme string
}

func (st *listenerStream) Open(flag int) (DeadlineReadWriteCloser, error) {
	return st.ln.Accept()
}

func (st *listenerStream) Close() error {
	return st.ln.Close()
}

func (st *listenerStream) Scheme() string {
	return st.scheme
}

func (st *listenerStream) String() string {
	return st.ln.Addr().String()
}

var (
	streamPid       uint32
	streamCounter   uint32
	streamCounterMu sync.Mutex
)

func getStreamName() string {
	streamCounterMu.Lock()
	// first run
	if streamCounter == 0 {
		streamPid = uint32(os.Getpid())
		streamCounter = 1
	}

	// counter (1~20bit) + pid (1~32bit) + random (12bit)
	streamId := (uint64(streamCounter)<<max(1, bits.Len32(streamPid))|uint64(streamPid))<<12 | rand.Uint64N(1<<12)

	streamCounter++
	if streamCounter >= 1<<20 {
		streamCounter = 1
	}
	streamCounterMu.Unlock()

	return "wooperstream-" + strconv.FormatUint(streamId, 36)
}
