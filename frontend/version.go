package frontend

import (
	"strings"
	"time"
)

// BuildVersion is populated by release builds through -ldflags. Development
// builds fall back to the Go VCS metadata embedded by the toolchain.
var BuildVersion string

// BuildTimestamp is populated by CI builds through -ldflags in RFC 3339 form.
// Builds without it fall back to the VCS commit time.
var BuildTimestamp string

func currentApplicationVersion() string {
	return applicationVersion(BuildVersion, currentDebugBuildReport())
}

func applicationVersion(override string, build debugBuildReport) string {
	if version := formatInjectedVersion(override); version != "" {
		return version
	}
	if version := strings.TrimSpace(build.Main.Version); version != "" &&
		version != "(devel)" {
		return shorten(version, 100)
	}
	if revision := strings.TrimSpace(build.Revision); revision != "" {
		if len(revision) > 7 {
			revision = revision[:7]
		}
		version := "Development " + revision
		if build.Modified {
			version += " (modified)"
		}
		return version
	}
	return "Development build"
}

func currentNightlyBuildStamp() string {
	return nightlyBuildStamp(
		BuildVersion,
		BuildTimestamp,
		currentDebugBuildReport(),
	)
}

// nightlyBuildStamp reports when a Nightly binary was produced. Stable
// releases return an empty stamp because their version already identifies
// the build.
func nightlyBuildStamp(override, timestamp string, build debugBuildReport) string {
	if !strings.HasPrefix(formatInjectedVersion(override), "Nightly") {
		return ""
	}
	raw := strings.TrimSpace(timestamp)
	if raw == "" {
		raw = strings.TrimSpace(build.Time)
	}
	if raw == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return shorten(raw, 40)
	}
	return parsed.UTC().Format("2006-01-02 15:04 UTC")
}

func formatInjectedVersion(value string) string {
	value = shorten(strings.TrimSpace(value), 100)
	switch {
	case strings.HasPrefix(value, "Nightly-"):
		return "Nightly " + strings.TrimPrefix(value, "Nightly-")
	case strings.HasPrefix(value, "Development-"):
		return "Development " + strings.TrimPrefix(value, "Development-")
	default:
		return value
	}
}
