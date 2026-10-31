// Command apiserver runs the in-process Quorum test server as a standalone
// process, so client test suites in other languages can exercise the real
// server under the same OpenAPI validation the Go suite gets.
//
// It prints the server URL as the first line of stdout, serves until it
// receives SIGINT or SIGTERM or its stdin is closed (whichever the test
// runner finds easier to do), and then exits non-zero if any traffic
// disagreed with the OpenAPI document or any documented operation was never
// called.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"

	"github.com/lawale/quorum/internal/apitest"
)

func main() {
	if os.Getenv("APISERVER_VERBOSE") == "" {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	}

	var (
		mu         sync.Mutex
		violations []string
	)
	report := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		mu.Lock()
		violations = append(violations, msg)
		mu.Unlock()
		fmt.Fprintln(os.Stderr, "spec violation: "+msg)
	}

	srv, err := apitest.Start(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apiserver:", err)
		os.Exit(2)
	}
	fmt.Println(srv.URL)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	stdinClosed := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		close(stdinClosed)
	}()
	select {
	case <-stop:
	case <-stdinClosed:
	}
	srv.Close()

	failed := false
	mu.Lock()
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "%d request(s) disagreed with the OpenAPI document\n", len(violations))
		failed = true
	}
	mu.Unlock()

	if uncovered := apitest.Uncovered(); len(uncovered) > 0 {
		sort.Strings(uncovered)
		fmt.Fprintln(os.Stderr, "operations in the OpenAPI document that the suite never called:")
		for _, op := range uncovered {
			fmt.Fprintln(os.Stderr, "  "+op)
		}
		failed = true
	}
	if failed {
		os.Exit(1)
	}
}
