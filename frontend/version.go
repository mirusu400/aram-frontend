package frontend

import (
	"strconv"
	"strings"
	"time"
)

// BuildVersion is populated by release builds through -ldflags. Development
// builds fall back to the Go VCS metadata embedded by the toolchain.
var BuildVersion string

// BuildTimestamp is populated by CI builds through -ldflags in RFC 3339 form.
// Builds without it fall back to the VCS commit time.
var BuildTimestamp string

// SelfUpdateDisabled is set to a non-empty value by store builds - the Google
// Play flavor, for example - through -ldflags. App-store policy forbids an app
// from checking for, offering, or installing its own updates outside the
// store, so those builds switch the whole self-update subsystem off. Sideloaded
// Stable/Nightly builds leave it empty and keep the in-app updater.
var SelfUpdateDisabled string

// selfUpdateDisabled reports whether the self-update subsystem is switched off
// for this build. Any value other than the empty string or an explicit
// off/false/0/no disables it, so a bare "-X ...SelfUpdateDisabled=1" is enough.
func selfUpdateDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(SelfUpdateDisabled)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

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

// runningReleaseChannel reports the channel of the running build and whether
// this is a published release build at all. Development and untagged builds
// return ok=false so the startup update check stays silent for them: the user
// asked for the notice only on a Stable or Nightly build.
func runningReleaseChannel() (updateChannel, bool) {
	injected := formatInjectedVersion(BuildVersion)
	switch {
	case strings.HasPrefix(injected, "Nightly "):
		return updateChannelNightly, true
	case injected == "" || strings.HasPrefix(injected, "Development"):
		return updateChannelStable, false
	default:
		return updateChannelStable, true
	}
}

// updateIsNewer reports whether latest is a newer build than current on the
// channel. Nightly versions embed unordered commit SHAs, so any difference
// from the running build means a newer Nightly exists; Stable versions are
// ordered semver tags and only a strictly greater one counts.
func updateIsNewer(current, latest string, channel updateChannel) bool {
	current = strings.TrimSpace(current)
	latest = strings.TrimSpace(latest)
	if current == "" || latest == "" {
		return false
	}
	if channel == updateChannelNightly {
		return latest != current
	}
	if order, ok := compareStableVersions(latest, current); ok {
		return order > 0
	}
	return latest != current
}

// compareStableVersions compares two dotted numeric version tags (an optional
// leading v and any -prerelease/+build suffix are ignored). It returns a
// sign like strings.Compare and ok=false when either side is not numeric, so
// the caller can fall back to a plain inequality test.
func compareStableVersions(left, right string) (int, bool) {
	leftFields, leftOK := stableVersionFields(left)
	rightFields, rightOK := stableVersionFields(right)
	if !leftOK || !rightOK {
		return 0, false
	}
	for index := 0; index < len(leftFields) || index < len(rightFields); index++ {
		var leftValue, rightValue int
		if index < len(leftFields) {
			leftValue = leftFields[index]
		}
		if index < len(rightFields) {
			rightValue = rightFields[index]
		}
		if leftValue != rightValue {
			if leftValue < rightValue {
				return -1, true
			}
			return 1, true
		}
	}
	return 0, true
}

func stableVersionFields(value string) ([]int, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	value = strings.TrimPrefix(value, "V")
	if cut := strings.IndexAny(value, "-+"); cut >= 0 {
		value = value[:cut]
	}
	if value == "" {
		return nil, false
	}
	parts := strings.Split(value, ".")
	fields := make([]int, 0, len(parts))
	for _, part := range parts {
		number, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, false
		}
		fields = append(fields, number)
	}
	return fields, true
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
