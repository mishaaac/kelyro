// Package version exposes build metadata for the Kelyro executable.
package version

import "fmt"

// These values are variables so release builds can replace them with -ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Info describes a Kelyro build without consulting Git at runtime.
type Info struct {
	Version string
	Commit  string
	Date    string
}

// Current returns the metadata embedded in the running build.
func Current() Info {
	return Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
	}
}

// IsDevelopment reports whether value is one of Kelyro's exact development
// build identifiers. Release versions are classified separately by the update
// package's SemVer parser.
func IsDevelopment(value string) bool {
	return value == "dev" || value == "unknown"
}

// String returns a stable human-readable representation of build metadata.
func (i Info) String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", i.Version, i.Commit, i.Date)
}
