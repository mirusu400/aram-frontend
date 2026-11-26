package frontend

import (
	"testing"
	"time"
)

type clockRecordingBackend struct {
	NullBackend
	configured int
	settings   ClockSettings
}

func (b *clockRecordingBackend) ConfigureClock(settings ClockSettings) error {
	b.configured++
	b.settings = settings
	return nil
}

func newClockTestShell(t *testing.T, now time.Time) (*Shell, *clockRecordingBackend) {
	t.Helper()
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	previous := handsetClockNow
	handsetClockNow = func() time.Time { return now }
	t.Cleanup(func() { handsetClockNow = previous })
	backend := &clockRecordingBackend{}
	shell := NewShell(backend, nil, "")
	shell.settings.Language = string(LanguageEnglish)
	return shell, backend
}

func clockRow(t *testing.T, shell *Shell, label string) *settingsRowModel {
	t.Helper()
	u := &shellUI{settingsSection: "General"}
	rows := u.settingsRowModels(shell)
	for index := range rows {
		if rows[index].label == label {
			return &rows[index]
		}
	}
	return nil
}

// The handset clock follows the device unless the user sets it apart, and the
// backend is told so at startup: a backend that is never configured keeps its
// fixed test date.
func TestHandsetClockFollowsTheDeviceByDefault(t *testing.T) {
	now := time.Date(2026, time.November, 26, 14, 3, 20, 0, time.Local)
	shell, backend := newClockTestShell(t, now)

	if backend.configured != 1 || backend.settings.Offset != 0 {
		t.Fatalf("startup clock configuration = %d calls, offset %s", backend.configured, backend.settings.Offset)
	}
	mode := clockRow(t, shell, "Handset clock")
	if mode == nil || mode.dropdown == nil || mode.dropdown.count != 2 || mode.dropdown.value() != 0 {
		t.Fatalf("handset clock row = %#v", mode)
	}
	if got := mode.dropdown.label(0); got != "This device's time" {
		t.Fatalf("default mode label = %q", got)
	}
	if row := clockRow(t, shell, "Handset clock: year"); row != nil {
		t.Fatalf("date rows are shown while the clock follows the device: %#v", row)
	}
}

// A custom time is an offset from the device clock: the chosen date is what a
// title opened now reads, and it keeps running afterwards.
func TestHandsetClockCustomTimeIsAnOffsetFromTheDevice(t *testing.T) {
	now := time.Date(2026, time.November, 26, 14, 3, 20, 0, time.Local)
	shell, backend := newClockTestShell(t, now)

	clockRow(t, shell, "Handset clock").dropdown.apply(1)
	if !shell.settings.HandsetClockCustom || backend.settings.Offset != 0 {
		t.Fatalf("custom mode starts at the device time: custom %t offset %s",
			shell.settings.HandsetClockCustom, backend.settings.Offset)
	}
	year := clockRow(t, shell, "Handset clock: year")
	if year == nil || year.dropdown.label(year.dropdown.value()) != "2026" {
		t.Fatalf("year row = %#v", year)
	}
	year.dropdown.apply(2008 - handsetClockMinYear)
	clockRow(t, shell, "Handset clock: month").dropdown.apply(3 - 1)
	clockRow(t, shell, "Handset clock: day").dropdown.apply(15 - 1)
	clockRow(t, shell, "Handset clock: hour").dropdown.apply(9)
	clockRow(t, shell, "Handset clock: minute").dropdown.apply(30)

	want := time.Date(2008, time.March, 15, 9, 30, 20, 0, time.Local)
	if got := shell.handsetClockTime(); !got.Equal(want) {
		t.Fatalf("handset clock = %s, want %s", got, want)
	}
	if got := backend.settings.Offset; got != want.Sub(now) {
		t.Fatalf("backend offset = %s, want %s", got, want.Sub(now))
	}

	// One device day later the handset is one day later too.
	later := now.Add(24 * time.Hour)
	handsetClockNow = func() time.Time { return later }
	if got := shell.handsetClockTime(); !got.Equal(want.Add(24 * time.Hour)) {
		t.Fatalf("handset clock a day later = %s, want %s", got, want.Add(24*time.Hour))
	}

	// The setting survives a restart.
	restarted := NewShell(backend, nil, "")
	if got := restarted.handsetClockTime(); !got.Equal(want.Add(24 * time.Hour)) {
		t.Fatalf("handset clock after restart = %s", got)
	}

	clockRow(t, shell, "Handset clock").dropdown.apply(0)
	if shell.settings.HandsetClockCustom || shell.settings.HandsetClockOffsetSeconds != 0 ||
		backend.settings.Offset != 0 {
		t.Fatalf("back to device time: %#v offset %s", shell.settings.HandsetClockCustom, backend.settings.Offset)
	}
}

// Choosing a shorter month pulls the day back instead of spilling into the
// next month, and the day row offers exactly that month's days.
func TestHandsetClockClampsTheDayToTheMonth(t *testing.T) {
	now := time.Date(2026, time.January, 31, 12, 0, 0, 0, time.Local)
	shell, _ := newClockTestShell(t, now)
	shell.setHandsetClockCustom(true)

	before := shell.handsetClockSignature()
	clockRow(t, shell, "Handset clock: month").dropdown.apply(2 - 1)
	if got := shell.handsetClockTime(); got.Month() != time.February || got.Day() != 28 {
		t.Fatalf("31 January moved to February = %s", got)
	}
	if day := clockRow(t, shell, "Handset clock: day"); day.dropdown.count != 28 {
		t.Fatalf("February day choices = %d, want 28", day.dropdown.count)
	}
	if shell.handsetClockSignature() == before {
		t.Fatal("the panel signature must change when the month length changes")
	}
	clockRow(t, shell, "Handset clock: year").dropdown.apply(2028 - handsetClockMinYear)
	if day := clockRow(t, shell, "Handset clock: day"); day.dropdown.count != 29 {
		t.Fatalf("leap February day choices = %d, want 29", day.dropdown.count)
	}
}
