package build

// Version string -ldflags "-X eventrelay/version.version=abcdefg"
var version string

// Exported method for returning the version string
func Version() string {
	if version == "" {
		return "n/a"
	}
	return version
}
