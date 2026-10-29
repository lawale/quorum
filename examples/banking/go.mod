module github.com/lawale/quorum/examples/banking

go 1.26.10

require (
	github.com/jackc/pgx/v5 v5.11.0
	github.com/lawale/quorum v0.0.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

// The example lives in the Quorum repository and tracks the client there.
// Outside this repository, drop the replace and require a released version.
replace github.com/lawale/quorum => ../..
