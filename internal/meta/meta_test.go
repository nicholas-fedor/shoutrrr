package meta

import (
	"regexp"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

// setBuildVars overrides the package-level build variables for one test and
// restores them on cleanup. Callers must not run in parallel, because the
// variables are shared package state.
func setBuildVars(t *testing.T, version, commit, date string) {
	t.Helper()

	origVersion, origCommit, origDate := Version, Commit, Date

	t.Cleanup(func() {
		Version, Commit, Date = origVersion, origCommit, origDate
	})

	Version, Commit, Date = version, commit, date
}

func TestGetVersionInfo(t *testing.T) { //nolint:paralleltest // Mutates package-global build variables.
	tests := []struct {
		name         string
		version      string
		commit       string
		date         string
		expect       Info
		partialMatch bool
	}{
		{
			name:    "GoReleaser build",
			version: "0.0.1",
			commit:  "abc123456789",
			date:    "2025-05-07T00:00:00Z",
			expect: Info{
				Version: "v0.0.1",
				Commit:  "abc1234",
				Date:    "2025-05-07",
			},
			partialMatch: false,
		},
		{
			name:    "Source build with default values",
			version: devVersion,
			commit:  unknownValue,
			date:    unknownValue,
			expect: Info{
				Version: unknownValue,
				Commit:  unknownValue,
				Date:    time.Now().UTC().Format("2006-01-02"),
			},
			partialMatch: true,
		},
		{
			name:    "Source build with empty values",
			version: "",
			commit:  "",
			date:    "",
			expect: Info{
				Version: unknownValue,
				Commit:  unknownValue,
				Date:    time.Now().UTC().Format("2006-01-02"),
			},
			partialMatch: false,
		},
		{
			name:    "Invalid GoReleaser version",
			version: "v",
			commit:  "",
			date:    "",
			expect: Info{
				Version: unknownValue,
				Commit:  unknownValue,
				Date:    time.Now().UTC().Format("2006-01-02"),
			},
			partialMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBuildVars(t, tt.version, tt.commit, tt.date)

			info := GetMetaInfo()

			if !tt.partialMatch {
				if info.Version != tt.expect.Version {
					t.Errorf("Version = %q, want %q", info.Version, tt.expect.Version)
				}

				if info.Commit != tt.expect.Commit {
					t.Errorf("Commit = %q, want %q", info.Commit, tt.expect.Commit)
				}

				// Validate Date format (YYYY-MM-DD) instead of exact match
				if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(info.Date) {
					t.Errorf("Date = %q, want valid YYYY-MM-DD format", info.Date)
				}
			} else if info.Version != tt.expect.Version && !strings.Contains(info.Version, "+dirty") {
				t.Errorf("Version = %q, want %q or dirty variant", info.Version, tt.expect.Version)
			}
		})
	}
}

func TestGetVersionInfo_VCSData(t *testing.T) { //nolint:paralleltest // Mutates package-global build variables.
	setBuildVars(t, devVersion, unknownValue, unknownValue)

	info := GetMetaInfo()

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		var vcsRevision, vcsTime, vcsModified string

		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				vcsRevision = setting.Value
			case "vcs.time":
				vcsTime = setting.Value
			case "vcs.modified":
				vcsModified = setting.Value
			}
		}

		if vcsRevision != "" {
			expectedCommit := vcsRevision
			if len(vcsRevision) >= 7 {
				expectedCommit = vcsRevision[:7]
			}

			if info.Commit == unknownValue {
				t.Errorf(
					"Expected commit %q, got %q; ensure repository has commit history",
					expectedCommit,
					info.Commit,
				)
			} else if info.Commit != expectedCommit {
				t.Errorf("Commit = %q, want %q", info.Commit, expectedCommit)
			}
		} else {
			t.Logf(
				"No vcs.revision found; ensure repository has Git metadata to cover commit assignment",
			)
		}

		if vcsTime != "" {
			if parsedTime, err := time.Parse(time.RFC3339, vcsTime); err == nil {
				expectedDate := parsedTime.Format("2006-01-02")
				if info.Date == unknownValue {
					t.Errorf(
						"Expected date %q, got %q; ensure vcs.time is a valid RFC3339 timestamp",
						expectedDate,
						info.Date,
					)
				} else if info.Date != expectedDate {
					t.Errorf("Date = %q, want %q", info.Date, expectedDate)
				}
			} else {
				t.Logf("vcs.time %q is invalid; date should be in YYYY-MM-DD format", vcsTime)

				if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(info.Date) {
					t.Errorf("Date = %q, want valid YYYY-MM-DD format", info.Date)
				}
			}
		} else {
			t.Logf("No vcs.time found; date should be in YYYY-MM-DD format")

			if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(info.Date) {
				t.Errorf("Date = %q, want valid YYYY-MM-DD format", info.Date)
			}
		}

		if vcsModified == trueValue && info.Version != unknownValue {
			if !strings.Contains(info.Version, "+dirty") {
				t.Errorf(
					"Expected version to contain '+dirty', got %q; ensure repository has uncommitted changes",
					info.Version,
				)
			}
		} else if vcsModified != trueValue {
			t.Logf(
				"Repository is clean (vcs.modified=%q); make uncommitted changes to cover '+dirty' case",
				vcsModified,
			)
		}
	} else {
		t.Logf(
			"debug.ReadBuildInfo() failed; ensure tests run in a Git repository to cover VCS parsing",
		)
	}
}

func TestGetVersionInfo_InvalidVCSTime(t *testing.T) { //nolint:paralleltest // Mutates package-global build variables.
	setBuildVars(t, devVersion, unknownValue, unknownValue)

	info := GetMetaInfo()

	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(info.Date) {
		t.Errorf("Date = %q, want valid YYYY-MM-DD format", info.Date)
	}
}

func TestUserAgent(t *testing.T) { //nolint:paralleltest // Mutates package-global Version.
	setBuildVars(t, "dev", Commit, Date)

	if got := UserAgent(); got != "shoutrrr/dev" {
		t.Errorf("UserAgent() = %q, want %q", got, "shoutrrr/dev")
	}
}

func TestGetMetaStr(t *testing.T) { //nolint:paralleltest // Mutates package-global build variables.
	tests := []struct {
		name    string
		version string
		commit  string
		date    string
		expect  string
	}{
		{
			name:    "With commit (GoReleaser build)",
			version: "0.8.10",
			commit:  "a6fcf77abcdef",
			date:    "2025-05-27T00:00:00Z",
			expect:  "v0.8.10 (Built on 2025-05-27 from Git SHA a6fcf77)",
		},
		{
			name:    "Without commit (go install build)",
			version: "0.8.10",
			commit:  unknownValue,
			date:    unknownValue,
			expect:  "v0.8.10 (Built on " + time.Now().UTC().Format("2006-01-02") + ")",
		},
		{
			name:    "Invalid version",
			version: "v",
			commit:  unknownValue,
			date:    unknownValue,
			expect:  "unknown (Built on " + time.Now().UTC().Format("2006-01-02") + ")",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBuildVars(t, tt.version, tt.commit, tt.date)

			result := GetMetaStr()
			if !strings.HasPrefix(result, strings.Split(tt.expect, " (")[0]) ||
				!regexp.MustCompile(`\d{4}-\d{2}-\d{2}`).MatchString(result) {
				t.Errorf("GetMetaStr() = %q, want format like %q", result, tt.expect)
			}
		})
	}
}

func TestContains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "Substring found",
			s:        "v1.0.0+dirty",
			substr:   "+dirty",
			expected: true,
		},
		{
			name:     "Substring not found",
			s:        "v1.0.0",
			substr:   "+dirty",
			expected: false,
		},
		{
			name:     "Empty string",
			s:        "",
			substr:   "+dirty",
			expected: false,
		},
		{
			name:     "Empty substring",
			s:        "v1.0.0",
			substr:   "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}
