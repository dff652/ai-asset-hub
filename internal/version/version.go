// Package version carries the build identity of this binary.
//
// A migration tool writes to other people's home directories, so every report
// and every deployment record has to be able to answer "which binary did
// this?". The values are injected at link time by scripts/build.sh; a plain
// `go build` leaves the dev defaults, which is itself informative.
package version

import "strings"

var (
	// Version is the release version, e.g. "0.1.0". "dev" for unreleased builds.
	Version = "dev"
	// Commit is the git commit the binary was built from.
	Commit = ""
	// Date is the build date in RFC3339, UTC.
	Date = ""
)

// ProducedBy identifies this binary in machine-readable output. It is stable
// for a given binary, so it never breaks build determinism.
func ProducedBy() string {
	if Commit == "" {
		return "aiah " + Version
	}
	return "aiah " + Version + "+" + shortCommit()
}

// Full is the human-readable form for `aiah version`.
func Full() string {
	parts := []string{"aiah " + Version}
	if Commit != "" {
		parts = append(parts, "commit "+shortCommit())
	}
	if Date != "" {
		parts = append(parts, "built "+Date)
	}
	return strings.Join(parts, ", ")
}

func shortCommit() string {
	if len(Commit) > 12 {
		return Commit[:12]
	}
	return Commit
}
