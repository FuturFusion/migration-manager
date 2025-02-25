package version

const Version string = "0.0.0-pre.2"

// GoVersion returns the project version formatted as a Go semantic version string.
func GoVersion() string {
	return "v" + Version
}
