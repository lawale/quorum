//go:build !embed

package widgets

import "net/http"

// Handler returns nil when built without the embed tag.
// The server checks for nil and skips mounting the assets route.
func Handler() http.Handler {
	return nil
}
