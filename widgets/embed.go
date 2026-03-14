//go:build embed

package widgets

import (
	_ "embed"
	"net/http"
)

//go:embed frontend/dist/embed.js
var embedJS []byte

// Handler returns an http.Handler that serves the embedded widget JS bundle.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(embedJS)
	})
}
