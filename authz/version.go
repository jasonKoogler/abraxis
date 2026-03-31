package authz

// Version information
const (
	// Version is the current version of the library
	Version = "0.3.0"
	
	// VersionPrerelease is a pre-release marker for the version
	// If this is "" (empty string) then it means that it is a final release.
	// Otherwise, this is a pre-release such as "dev", "beta", "alpha", etc.
	VersionPrerelease = ""
)

// GetVersion returns the full version string
func GetVersion() string {
	if VersionPrerelease != "" {
		return Version + "-" + VersionPrerelease
	}
	return Version
}
