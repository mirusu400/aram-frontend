package frontend

import "testing"

func TestUpdateIsNewer(t *testing.T) {
	cases := []struct {
		name    string
		current string
		latest  string
		channel updateChannel
		want    bool
	}{
		{"nightly differs", "Nightly aaaaaaa-cbbbbbbb-fccccccc", "Nightly ddddddd-ceeeeeee-fffffff", updateChannelNightly, true},
		{"nightly same", "Nightly aaaaaaa", "Nightly aaaaaaa", updateChannelNightly, false},
		{"nightly empty latest", "Nightly aaaaaaa", "", updateChannelNightly, false},
		{"stable greater", "v1.2.3", "v1.2.4", updateChannelStable, true},
		{"stable minor greater", "v1.2.3", "v1.3.0", updateChannelStable, true},
		{"stable equal", "v1.2.3", "v1.2.3", updateChannelStable, false},
		{"stable lower", "v1.2.3", "v1.2.2", updateChannelStable, false},
		{"stable no v prefix", "1.2.3", "1.2.10", updateChannelStable, true},
		{"stable prerelease ignored equal", "v1.2.3", "v1.2.3-rc1", updateChannelStable, false},
		{"stable unparseable falls back to inequality", "weekly", "monthly", updateChannelStable, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := updateIsNewer(tc.current, tc.latest, tc.channel); got != tc.want {
				t.Fatalf("updateIsNewer(%q, %q, %q) = %t, want %t",
					tc.current, tc.latest, tc.channel, got, tc.want)
			}
		})
	}
}

func TestRunningReleaseChannel(t *testing.T) {
	previous := BuildVersion
	t.Cleanup(func() { BuildVersion = previous })

	cases := []struct {
		build       string
		wantChannel updateChannel
		wantOK      bool
	}{
		{"", updateChannelStable, false},
		{"Development-abcdef0-c1234567-f89abcde", updateChannelStable, false},
		{"Nightly-abcdef0-c1234567-f89abcde", updateChannelNightly, true},
		{"v1.4.0", updateChannelStable, true},
	}
	for _, tc := range cases {
		BuildVersion = tc.build
		channel, ok := runningReleaseChannel()
		if ok != tc.wantOK || (ok && channel != tc.wantChannel) {
			t.Fatalf("runningReleaseChannel() for %q = (%q, %t), want (%q, %t)",
				tc.build, channel, ok, tc.wantChannel, tc.wantOK)
		}
	}
}

func TestConsumeUpdateCheckResultRaisesNoticeOnlyWhenNewer(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)

	shell := NewShell(NullBackend{}, nil, "")

	// A build with the same version as the running app raises no notice.
	shell.consumeUpdateCheckResult(updateCheckResult{
		component: updateComponentProduct,
		channel:   updateChannelNightly,
		version:   currentApplicationVersion(),
	})
	if shell.updateNoticeReady {
		t.Fatal("an identical version should not raise the update notice")
	}

	// A different Nightly build does.
	shell.consumeUpdateCheckResult(updateCheckResult{
		component: updateComponentProduct,
		channel:   updateChannelNightly,
		version:   "Nightly deadbee-c1234567-f89abcde",
	})
	if !shell.updateNoticeReady {
		t.Fatal("a newer Nightly build should raise the update notice")
	}
	if shell.updateNoticeVersion != "Nightly deadbee-c1234567-f89abcde" {
		t.Fatalf("notice version = %q", shell.updateNoticeVersion)
	}

	// A failed check never raises the notice and never panics.
	shell.updateNoticeReady = false
	shell.consumeUpdateCheckResult(updateCheckResult{
		component: updateComponentProduct,
		channel:   updateChannelNightly,
		err:       errContext,
	})
	if shell.updateNoticeReady {
		t.Fatal("a failed check must not raise the update notice")
	}
}

var errContext = &checkError{"network unreachable"}

type checkError struct{ message string }

func (e *checkError) Error() string { return e.message }
