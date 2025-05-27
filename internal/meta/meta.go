package meta

import (
	"fmt"
	"runtime/debug"
	"time"
)

// Constants for repeated string values.
const (
	devVersion      = "dev"
	unknownValue    = "unknown"
	trueValue       = "true"
	commitSHALength = 7 // Length to shorten Git commit SHA
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

// GetMetaStr returns the formatted version string, including commit info only if available.
func GetMetaStr() string {
	version := GetVersion()
	date := GetDate()
	commit := GetCommit()

	if commit == unknownValue {
		return fmt.Sprintf("%s (Built on %s)", version, date)
	}

	return fmt.Sprintf("%s (Built on %s from Git SHA %s)", version, date, commit)
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
	// Return Commit if set by GoReleaser (non-empty and not "unknown")
	if Commit != unknownValue && Commit != "" {
		if len(Commit) >= commitSHALength {
			return Commit[:commitSHALength]
		}

		return Commit
	}

	// Try to get commit from debug.ReadBuildInfo for source builds
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && setting.Value != "" {
				if len(setting.Value) >= commitSHALength {
					return setting.Value[:commitSHALength]
				}

				return setting.Value
			}
		}
	}

	// Fallback to unknown if no commit is found
	return unknownValue
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
		// Fallback to current date if no VCS time is available
		return time.Now().UTC().Format("2006-01-02")
	}

	// Shorten date if provided by GoReleaser
	if date != "" && date != unknownValue {
		if t, err := time.Parse(time.RFC3339, date); err == nil {
			return t.Format("2006-01-02") // Shorten to YYYY-MM-DD
		}
	}

	// Fallback to current date if date is invalid
	return time.Now().UTC().Format("2006-01-02")
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
