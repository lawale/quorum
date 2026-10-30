package client

import (
	"runtime/debug"
	"sync"
)

const modulePath = "github.com/lawale/quorum"

// Version is the version of the Quorum module this client was built from, as
// resolved by the Go toolchain (for example "v0.4.0"), and the value sent in
// the default User-Agent. A Go module's version is its git tag, so it is the
// same version as the matching Docker image and npm package. It reads "dev"
// when the module is being built from a checkout or through a replace
// directive, where no release version exists.
func Version() string {
	versionOnce.Do(func() { version = resolveVersion(debug.ReadBuildInfo()) })
	return version
}

var (
	versionOnce sync.Once
	version     string
)

func resolveVersion(bi *debug.BuildInfo, ok bool) string {
	if !ok || bi == nil {
		return "dev"
	}
	for _, dep := range bi.Deps {
		if dep.Path != modulePath {
			continue
		}
		if dep.Replace != nil || dep.Version == "" || dep.Version == "(devel)" {
			return "dev"
		}
		return dep.Version
	}
	// The client is being built inside the Quorum module itself.
	return "dev"
}
