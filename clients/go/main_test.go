package client_test

import (
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/lawale/quorum/internal/apitest"
)

// The suite runs against the real server in-process, with every request and
// response validated against api/openapi.yaml. After the run, any operation
// the document describes that no test reached is a hole in the contract.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 {
		if missed := apitest.Uncovered(); len(missed) > 0 {
			sort.Strings(missed)
			fmt.Fprintf(os.Stderr, "operations in openapi.yaml not exercised by the client tests:\n")
			for _, op := range missed {
				fmt.Fprintf(os.Stderr, "  %s\n", op)
			}
			code = 1
		}
	}
	os.Exit(code)
}
