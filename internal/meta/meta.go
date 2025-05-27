package meta

import (
	"runtime/debug"
	"time"
)

// Constants for repeated string values.
const (
	devVersion   = "dev"
	unknownValue = "unknown"
	trueValue    = "true"
)

// These values are populated by GoReleaser during release builds.
var (
	// Version is the Shoutrrr version (e.g., "v0.0.1").
	Version = devVersion
	// Commit is the Git commit SHA (e.g., "abc123").
	Commit = unknownValue
	// Date is the build or commit timestamp in RFC3339 format (e.g., "2025-05-07T00:00:00Z").
	Date = unknownValue
)

// Info holds version information for Shoutrrr.
type Info struct {
	Version string
	Commit  string
	Date    string
}

// GetVersion returns the version string, using debug.ReadBuildInfo for source builds
// or GoReleaser variables for release builds.
func GetVersion() string {
	version := Version

	// If building from source (not GoReleaser), try to get version from debug.ReadBuildInfo
	if version == devVersion || version == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			// Get the module version (e.g., v1.1.4 or v1.1.4+dirty)
			version = info.Main.Version
			if version == "(devel)" || version == "" {
				version = devVersion
			}
			// Check for dirty state
			for _, setting := range info.Settings {
				if setting.Key == "vcs.modified" && setting.Value == trueValue &&
					version != unknownValue && !contains(version, "+dirty") {
					version += "+dirty"
				}
			}
		}
	} else {
		// GoReleaser provides a valid version without 'v' prefix, so add it
		if version != "" && version != "v" {
			version = "v" + version
		}
	}

	// Fallback default if still unset or invalid
	if version == "" || version == devVersion || version == "v" {
		return unknownValue
	}

	return version
}

// GetCommit returns the commit SHA, using debug.ReadBuildInfo for source builds
// or GoReleaser variables for release builds.
func GetCommit() string {
	commit := Commit

	// If building from source (not GoReleaser), try to get commit from debug.ReadBuildInfo
	if commit == unknownValue || commit == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					commit = setting.Value
					break
				}
			}
		}
	}

	// Shorten commit to 7 characters if it's a valid SHA
	if len(commit) >= 7 && commit != unknownValue {
		return commit[:7]
	}

	// Fallback default if still unset
	if commit == "" {
		return unknownValue
	}

	return commit
}

// GetDate returns the build or commit date, using debug.ReadBuildInfo for source builds
// or GoReleaser variables for release builds.
func GetDate() string {
	date := Date

	// If building from source (not GoReleaser), try to get date from debug.ReadBuildInfo
	if date == unknownValue || date == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.time" {
					if t, err := time.Parse(time.RFC3339, setting.Value); err == nil {
						return t.Format("2006-01-02") // Shorten to YYYY-MM-DD
					}
				}
			}
		}
	} else {
		// Shorten date if provided by GoReleaser
		if date != "" && date != unknownValue {
			if t, err := time.Parse(time.RFC3339, date); err == nil {
				return t.Format("2006-01-02") // Shorten to YYYY-MM-DD
			}
		}
	}

	// Fallback default if still unset
	if date == "" {
		return unknownValue
	}

	return date
}

// GetMetaInfo returns version information by combining GetVersion, GetCommit, and GetDate.
func GetMetaInfo() Info {
	return Info{
		Version: GetVersion(),
		Commit:  GetCommit(),
		Date:    GetDate(),
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
