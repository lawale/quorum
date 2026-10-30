// Package version holds the server build version.
package version

// Version is the release the server binary was built from. Release builds set
// it from the git tag with
//
//	-ldflags "-X github.com/lawale/quorum/internal/version.Version=v1.2.3"
//
// and it stays "dev" for local builds.
var Version = "dev"
