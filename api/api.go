// Package api carries Quorum's OpenAPI document. The server serves it and the
// test harness validates live traffic against it, so the document and the
// handlers cannot drift apart without a test failing.
package api

import _ "embed"

// Spec is the OpenAPI 3.0 description of the public HTTP API.
//
//go:embed openapi.yaml
var Spec []byte
