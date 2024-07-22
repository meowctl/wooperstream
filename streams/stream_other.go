//go:build !(unix || windows)

package streams

import "net"

const CurrentStreamImpl = TCPSocketStreamImpl

func NewStream() (Stream, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return nil, err
	}
	return FromListener(ln, "tcp")
}
