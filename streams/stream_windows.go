package streams

import (
	"path/filepath"

	"github.com/Microsoft/go-winio"
)

const CurrentStreamImpl = Win32PipeStreamImpl

func NewStream() (Stream, error) {
	ln, err := winio.ListenPipe(filepath.Join(`\\.\pipe`, getStreamName()), nil)
	if err != nil {
		return nil, err
	}
	return FromListener(ln, "file")
}
