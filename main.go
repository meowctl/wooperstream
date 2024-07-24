package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/meowctl/wooperstream/streams"
)

var (
	ServerModes = [...]string{
		os.O_RDONLY: "r",
		os.O_WRONLY: "w",
	}
	ServerModeMap = map[string]int{
		"r": os.O_RDONLY,
		"w": os.O_WRONLY,
	}
)

var ErrInvalidMode = fmt.Errorf("invalid mode")

func main() {
	os.Exit(run())
}

func run() (code int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "select a mode: %s\n", strings.Join(ServerModes[:], ", "))
		return 1
	}
	mode, ok := ServerModeMap[os.Args[1]]
	if !ok {
		fmt.Fprintln(os.Stderr, "invalid mode")
		return 1
	}
	log.Printf("current implementation: %s\n", streams.CurrentStreamImpl)
	for {
		if err := startServer(ctx, mode); err != nil {
			if errors.Is(err, ctx.Err()) {
				log.Println("interrupted by user")
				return 2
			} else {
				log.Println(err)
			}
		}
	}
}

func supportsMode(mode int) bool {
	return mode >= 0 && mode < len(ServerModes)
}

func startServer(ctx context.Context, mode int) error {
	if !supportsMode(mode) {
		return ErrInvalidMode
	}
	st, err := streams.NewStream()
	if err != nil {
		return err
	}
	defer st.Close()
	log.Printf("ipc path: %s:%s\n", st.Scheme(), st)

	errCh, closerCh := make(chan error), make(chan io.Closer)
	go func() {
		errCh <- openAndRun(ctx, mode, st, closerCh)
	}()
	var closer io.Closer
	for {
		select {
		case closer = <-closerCh:
		case err := <-errCh:
			return err
		case <-ctx.Done():
			if closer != nil {
				closer.Close()
			} else { // likely blocked on open
				st.Close()
			}
			if err := <-errCh; err != nil &&
				!(errors.Is(err, ctx.Err()) || errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed)) {
				log.Println(err)
			}
			return ctx.Err()
		}
	}
}

func openAndRun(ctx context.Context, mode int, st streams.Stream, closerCh chan<- io.Closer) error {
	if !supportsMode(mode) {
		return ErrInvalidMode
	}
	rdwr, err := st.Open(mode)
	if err != nil {
		return err
	}
	defer rdwr.Close()
	select {
	case closerCh <- rdwr:
	case <-ctx.Done():
		return ctx.Err()
	}
	switch mode {
	case os.O_RDONLY:
		return readServer(rdwr)
	case os.O_WRONLY:
		return writeServer(ctx, rdwr)
	}
	return nil
}

func readServer(r io.Reader) error {
	_, err := io.Copy(os.Stdout, r)
	return err
}

func writeServer(ctx context.Context, w io.Writer) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for i := 0; ; i++ {
		_, err := fmt.Fprintf(w, "[%d] %s - %d\n", i, time.Now().Format("02/01/2006 15:04:05 -07:00"), rand.Int())
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
