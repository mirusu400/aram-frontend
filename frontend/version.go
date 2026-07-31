package frontend

import "strings"

// BuildVersion is populated by release builds through -ldflags. Development
// builds fall back to the Go VCS metadata embedded by the toolchain.
var BuildVersion string

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
