package version

// Version information
// These can be overridden at build time using ldflags:
// go build -ldflags="-X reel/internal/version.Version=1.2.3"

var (
	Version   = "dev"      // Version number (e.g., "1.0.0")
	GitCommit = "unknown"  // Git commit hash
	BuildDate = "unknown"  // Build date
	GoVersion = "unknown"  // Go version used for build
)

// GetVersion returns the version string
func GetVersion() string {
	if Version == "dev" {
		return "dev-" + GitCommit[:7]
	}
	return Version
}

// GetUserAgent returns the user-agent string for HTTP clients
func GetUserAgent() string {
	return "Reel/" + GetVersion()
}

// GetFullVersion returns detailed version information
func GetFullVersion() string {
	return "Reel " + Version + " (commit: " + GitCommit + ", built: " + BuildDate + ", go: " + GoVersion + ")"
}
