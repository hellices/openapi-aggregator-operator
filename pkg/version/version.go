package version

import "fmt"

var (
	// version of the operator, injected during build
	version = "unknown"
	// buildDate is the build date of the operator, injected during build
	buildDate = "unknown"
)

// Version holds this Operator's version as well as build date
type Version struct {
	Operator  string `json:"operator"`
	BuildDate string `json:"build-date"`
}

// Get returns the Version object with all version information
func Get() Version {
	return Version{
		Operator:  version,
		BuildDate: buildDate,
	}
}

// String implements the fmt.Stringer interface
func (v Version) String() string {
	return fmt.Sprintf("Version(Operator='%v', BuildDate='%v')", v.Operator, v.BuildDate)
}
