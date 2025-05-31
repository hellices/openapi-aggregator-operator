package version

var (
	// version of the operator, injected during build
	version = "unknown"
	// buildDate is the build date of the operator, injected during build
	buildDate = "unknown"
)

// GetVersion returns the version of the operator
func GetVersion() string {
	return version
}

// GetBuildDate returns the build date of the operator
func GetBuildDate() string {
	return buildDate
}
