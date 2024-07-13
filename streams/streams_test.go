package streams

import (
	"testing"
)

func BenchmarkGetStreamName(b *testing.B) {
	for range b.N {
		getStreamName()
	}
}
