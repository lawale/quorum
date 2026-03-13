//go:build !console

package console

import "net/http"

// Handler returns nil when built without the console tag.
// The server checks for nil and skips mounting the SPA route.
func Handler() http.Handler {
	return nil
}
